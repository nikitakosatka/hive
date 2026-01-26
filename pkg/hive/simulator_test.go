package hive

import (
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nikitakosatka/hive/pkg/failure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimulator_AddNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*Simulator)
		nodeID      string
		expectError bool
		errorType   error
	}{
		{
			name:        "add single node",
			setup:       func(s *Simulator) {},
			nodeID:      "node1",
			expectError: false,
		},
		{
			name: "add duplicate node",
			setup: func(s *Simulator) {
				s.AddNode(NewBaseNode("node1"))
			},
			nodeID:      "node1",
			expectError: true,
			errorType:   ErrNodeAlreadyExists,
		},
		{
			name:        "add multiple nodes",
			setup:       func(s *Simulator) {},
			nodeID:      "node2",
			expectError: false,
		},
		{
			name:        "edge case: empty ID",
			setup:       func(s *Simulator) {},
			nodeID:      "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sim := NewSimulator(NewConfig())
			tt.setup(sim)

			node := NewBaseNode(tt.nodeID)

			err := sim.AddNode(node)
			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errorType)
			} else {
				require.NoError(t, err)

				retrieved, err := sim.GetNode(tt.nodeID)
				require.NoError(t, err)
				assert.Equal(t, node, retrieved)
			}
		})
	}
}

func TestSimulator_RemoveNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*Simulator)
		nodeID      string
		expectError bool
		errorType   error
	}{
		{
			name: "remove existing node",
			setup: func(s *Simulator) {
				s.AddNode(NewBaseNode("node1"))
			},
			nodeID:      "node1",
			expectError: false,
		},
		{
			name: "remove non-existent node",
			setup: func(s *Simulator) {
				// Don't add any nodes
			},
			nodeID:      "nonexistent",
			expectError: true,
			errorType:   ErrNodeNotFound,
		},
		{
			name: "remove from multiple nodes",
			setup: func(s *Simulator) {
				s.AddNode(NewBaseNode("node1"))
				s.AddNode(NewBaseNode("node2"))
				s.AddNode(NewBaseNode("node3"))
			},
			nodeID:      "node2",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sim := NewSimulator(NewConfig())
			tt.setup(sim)

			initialCount := sim.GetNodeCount()

			err := sim.RemoveNode(tt.nodeID)
			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errorType)
				assert.Equal(t, initialCount, sim.GetNodeCount())
			} else {
				require.NoError(t, err)
				assert.Equal(t, initialCount-1, sim.GetNodeCount())

				_, err = sim.GetNode(tt.nodeID)
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrNodeNotFound)
			}
		})
	}
}

func TestSimulator_Start(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*Simulator)
		startTwice  bool
		expectError bool
	}{
		{
			name: "start with nodes",
			setup: func(s *Simulator) {
				s.AddNode(NewBaseNode("node1"))
			},
			startTwice:  false,
			expectError: false,
		},
		{
			name: "start without nodes",
			setup: func(s *Simulator) {
			},
			startTwice:  false,
			expectError: false,
		},
		{
			name: "start twice",
			setup: func(s *Simulator) {
				s.AddNode(NewBaseNode("node1"))
			},
			startTwice:  true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sim := NewSimulator(NewConfig())
			tt.setup(sim)

			err := sim.Start()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, sim.IsRunning())

				if tt.startTwice {
					err = sim.Start()
					assert.NoError(t, err)
					assert.True(t, sim.IsRunning())
				}
			}
		})
	}
}

func TestSimulator_Stop(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setup     func(*Simulator)
		stopTwice bool
	}{
		{
			name: "stop running simulator",
			setup: func(s *Simulator) {
				s.AddNode(NewBaseNode("node1"))
				s.Start()
			},
			stopTwice: false,
		},
		{
			name: "stop not running",
			setup: func(s *Simulator) {
				// Don't start
			},
			stopTwice: false,
		},
		{
			name: "stop twice",
			setup: func(s *Simulator) {
				s.AddNode(NewBaseNode("node1"))
				s.Start()
			},
			stopTwice: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sim := NewSimulator(NewConfig())
			tt.setup(sim)

			err := sim.Stop()
			assert.NoError(t, err)
			assert.False(t, sim.IsRunning())

			if tt.stopTwice {
				err = sim.Stop()
				assert.NoError(t, err)
			}
		})
	}
}

func TestSimulator_GetCrashedNodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func(*failure.Failure)
		expected map[string]bool
	}{
		{
			name:     "no crashed nodes",
			setup:    func(f *failure.Failure) {},
			expected: map[string]bool{},
		},
		{
			name: "single crashed node",
			setup: func(f *failure.Failure) {
				f.CrashNode("node1")
			},
			expected: map[string]bool{"node1": true},
		},
		{
			name: "multiple crashed nodes",
			setup: func(f *failure.Failure) {
				f.CrashNode("node1")
				f.CrashNode("node2")
			},
			expected: map[string]bool{"node1": true, "node2": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			failConfig := failure.NewCrashStop(0.0)
			config := NewConfig(WithNodesFailures(failConfig))
			sim := NewSimulator(config)

			tt.setup(failConfig)

			crashed := sim.GetCrashedNodes()
			assert.Equal(t, tt.expected, crashed)
		})
	}
}

func TestSimulator_MessageDelivery(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		sim := NewSimulator(NewConfig())

		testNode := newTestNode("node1")
		testNode2 := newTestNode("node2")

		sim.AddNode(testNode)
		sim.AddNode(testNode2)
		sim.Start()

		synctest.Wait()

		err := testNode.Send("node2", "hello")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		messages := testNode2.GetReceivedMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, "hello", messages[0].Payload)
		assert.Equal(t, "node1", messages[0].From)

		sim.Stop()
	})
}

func TestSimulator_NodeCrash_MessageDropped(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		failConfig := failure.NewCrashStop(0.0)
		config := NewConfig(WithNodesFailures(failConfig))
		sim := NewSimulator(config)

		testNode1 := newTestNode("node1")
		testNode2 := newTestNode("node2")

		sim.AddNode(testNode1)
		sim.AddNode(testNode2)
		sim.Start()

		synctest.Wait()

		failConfig.CrashNode("node2")
		testNode2.Stop()

		synctest.Wait()

		err := testNode1.Send("node2", "test")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		messages := testNode2.GetReceivedMessages()
		assert.Empty(t, messages)

		sim.Stop()
	})
}

func TestSimulator_GetPendingMessageCount(t *testing.T) {
	t.Parallel()
	sim := NewSimulator(NewConfig())
	node1 := NewBaseNode("node1")
	node2 := NewBaseNode("node2")

	sim.AddNode(node1)
	sim.AddNode(node2)
	sim.Start()
	defer sim.Stop()

	node1.SetSendFunc(func(to string, msg *Message) error {
		return sim.network.Send(node1.ID(), to, msg)
	})

	node1.Send("node2", "test")

	count := sim.GetPendingMessageCount()
	assert.GreaterOrEqual(t, count, 0)
}

func TestSimulator_ConcurrentOperations(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		sim := NewSimulator(NewConfig())

		var wg sync.WaitGroup
		numNodes := 10

		for i := 0; i < numNodes; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				node := NewBaseNode(fmt.Sprintf("node0%d", id))
				sim.AddNode(node)
			}(i)
		}

		synctest.Wait()
		wg.Wait()

		assert.Equal(t, numNodes, sim.GetNodeCount())
	})
}
