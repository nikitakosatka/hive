package hive

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nikitakosatka/hive/pkg/failure"
	"github.com/nikitakosatka/hive/pkg/network"
	timemodel "github.com/nikitakosatka/hive/pkg/time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetwork_RegisterNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		nodeID string
	}{
		{"single node", "node1"},
		{"multiple nodes", "node2"},
		{"edge case: empty ID", ""},
	}

	net := NewNetwork(
		network.NewReliableNetwork(),
		timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
		failure.NewCrashStop(0.0),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := NewBaseNode(tt.nodeID)
			net.RegisterNode(node)

			retrieved, exists := net.nodes[tt.nodeID]
			assert.True(t, exists)
			assert.Equal(t, node, retrieved)
		})
	}
}

func TestNetwork_UnregisterNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*Network)
		nodeID      string
		expectedLen int
	}{
		{
			name: "unregister single node",
			setup: func(n *Network) {
				n.RegisterNode(NewBaseNode("node1"))
			},
			nodeID:      "node1",
			expectedLen: 0,
		},
		{
			name: "unregister from multiple nodes",
			setup: func(n *Network) {
				n.RegisterNode(NewBaseNode("node1"))
				n.RegisterNode(NewBaseNode("node2"))
				n.RegisterNode(NewBaseNode("node3"))
			},
			nodeID:      "node2",
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			net := NewNetwork(
				network.NewReliableNetwork(),
				timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
				failure.NewCrashStop(0.0),
			)

			tt.setup(net)
			net.UnregisterNode(tt.nodeID)
			assert.Len(t, net.nodes, tt.expectedLen)
		})
	}
}

func TestNetwork_Send(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*Network, *failure.Failure)
		fromID      string
		toID        string
		expectError bool
		errorType   error
	}{
		{
			name: "crashed sender - error",
			setup: func(n *Network, f *failure.Failure) {
				f.CrashNode("sender")
			},
			fromID:      "sender",
			toID:        "receiver",
			expectError: true,
			errorType:   ErrNodeNotRunning,
		},
		{
			name: "crashed receiver - no error (silent drop)",
			setup: func(n *Network, f *failure.Failure) {
				f.CrashNode("receiver")
			},
			fromID:      "sender",
			toID:        "receiver",
			expectError: false,
		},
		{
			name: "normal send",
			setup: func(n *Network, f *failure.Failure) {
				// No crashes
			},
			fromID:      "sender",
			toID:        "receiver",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			failConfig := failure.NewCrashStop(0.0)
			net := NewNetwork(
				network.NewReliableNetwork(),
				timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
				failConfig,
			)

			tt.setup(net, failConfig)
			msg := NewMessage(tt.fromID, tt.toID, "test")

			err := net.Send(tt.fromID, tt.toID, msg)
			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errorType)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNetwork_Send_MessageDropped(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		netConfig := network.NewFairLossNetwork(1.0, 0.0) // 100% drop probability
		net := NewNetwork(
			netConfig,
			timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
			failure.NewCrashStop(0.0),
		)

		msg := NewMessage("sender", "receiver", "test")

		err := net.Send("sender", "receiver", msg)
		assert.NoError(t, err)

		time.Sleep(20 * time.Millisecond)
		synctest.Wait()
		net.ProcessMessages()
		assert.Equal(t, 0, net.GetPendingMessageCount())
	})
}

func TestNetwork_Send_MessageDelivered(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		net := NewNetwork(
			network.NewReliableNetwork(),
			timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
			failure.NewCrashStop(0.0),
		)

		testNode := newTestNode("receiver")
		testNode.Start(context.Background())
		defer testNode.Stop()
		testNode.SetNodeRef(testNode)
		net.RegisterNode(testNode)

		msg := NewMessage("sender", "receiver", "test")
		err := net.Send("sender", "receiver", msg)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
		synctest.Wait()
		net.ProcessMessages()

		time.Sleep(50 * time.Millisecond)
		synctest.Wait()

		messages := testNode.GetReceivedMessages()
		require.Len(t, messages, 1)
		assert.Equal(t, msg, messages[0])

		testNode.Stop()
	})
}

func TestNetwork_Send_MessageDuplicated(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		netConfig := network.NewFairLossNetwork(0.0, 1.0) // 100% duplicate probability
		net := NewNetwork(
			netConfig,
			timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
			failure.NewCrashStop(0.0),
		)

		testNode := newTestNode("receiver")
		testNode.Start(context.Background())
		defer testNode.Stop()
		testNode.SetNodeRef(testNode)
		net.RegisterNode(testNode)

		msg := NewMessage("sender", "receiver", "test")
		err := net.Send("sender", "receiver", msg)
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
		synctest.Wait()
		net.ProcessMessages()

		time.Sleep(50 * time.Millisecond)
		synctest.Wait()

		messages := testNode.GetReceivedMessages()
		assert.GreaterOrEqual(t, len(messages), 1)

		testNode.Stop()
	})
}

func TestNetwork_GetPendingMessageCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		latency        time.Duration
		numMessages    int
		expectedBefore int
		expectedAfter  int
	}{
		{
			name:           "single message",
			latency:        100 * time.Millisecond,
			numMessages:    1,
			expectedBefore: 0,
			expectedAfter:  1,
		},
		{
			name:           "multiple messages",
			latency:        50 * time.Millisecond,
			numMessages:    5,
			expectedBefore: 0,
			expectedAfter:  5,
		},
		{
			name:           "zero latency",
			latency:        0,
			numMessages:    3,
			expectedBefore: 0,
			expectedAfter:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			net := NewNetwork(
				network.NewReliableNetwork(),
				timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: tt.latency}, 0.0),
				failure.NewCrashStop(0.0),
			)

			assert.Equal(t, tt.expectedBefore, net.GetPendingMessageCount())

			for i := 0; i < tt.numMessages; i++ {
				msg := NewMessage("sender", "receiver", i)
				net.Send("sender", "receiver", msg)
			}

			assert.Equal(t, tt.expectedAfter, net.GetPendingMessageCount())
		})
	}
}

func TestNetwork_SetStartTime(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		net := NewNetwork(
			network.NewReliableNetwork(),
			timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
			failure.NewCrashStop(0.0),
		)

		newStartTime := time.Now().Add(-1 * time.Hour)
		net.SetStartTime(newStartTime)

		msg := NewMessage("sender", "receiver", "test")
		net.Send("sender", "receiver", msg)

		assert.Equal(t, 1, net.GetPendingMessageCount())
	})
}

func TestNetwork_ConcurrentSend(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		numMessages int
		goroutines  int
	}{
		{
			name:        "few messages, many goroutines",
			numMessages: 10,
			goroutines:  100,
		},
		{
			name:        "many messages, few goroutines",
			numMessages: 1000,
			goroutines:  10,
		},
		{
			name:        "balanced",
			numMessages: 100,
			goroutines:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				net := NewNetwork(
					network.NewReliableNetwork(),
					timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
					failure.NewCrashStop(0.0),
				)

				var wg sync.WaitGroup
				// Ensure we distribute messages evenly, with remainder going to first goroutines
				messagesPerGoroutine := tt.numMessages / tt.goroutines
				remainder := tt.numMessages % tt.goroutines

				for i := 0; i < tt.goroutines; i++ {
					wg.Add(1)
					go func(id int) {
						defer wg.Done()
						// First 'remainder' goroutines get one extra message
						count := messagesPerGoroutine
						if id < remainder {
							count++
						}
						for j := 0; j < count; j++ {
							msg := NewMessage("sender", "receiver", id*1000+j)
							net.Send("sender", "receiver", msg)
						}
					}(i)
				}

				wg.Wait()
				synctest.Wait()

				// All messages should be pending (not yet delivered due to latency)
				// Note: With synctest, time doesn't advance automatically, so messages stay pending
				assert.Equal(t, tt.numMessages, net.GetPendingMessageCount())
			})
		})
	}
}

func TestNetwork_ProcessMessages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		setup            func(*Network, *testNode)
		expectedMessages int
	}{
		{
			name: "node not running",
			setup: func(n *Network, node *testNode) {
				n.RegisterNode(node)
			},
			expectedMessages: 0,
		},
		{
			name: "node stopped after registration",
			setup: func(n *Network, node *testNode) {
				node.Start(context.Background())
				node.SetNodeRef(node)
				n.RegisterNode(node)
				node.Stop()
			},
			expectedMessages: 0,
		},
		{
			name: "node crashed",
			setup: func(n *Network, node *testNode) {
				node.Start(context.Background())
				node.SetNodeRef(node)
				n.RegisterNode(node)
				n.failConfig.CrashNode("receiver")
			},
			expectedMessages: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				net := NewNetwork(
					network.NewReliableNetwork(),
					timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0),
					failure.NewCrashStop(0.0),
				)

				testNode := newTestNode("receiver")
				tt.setup(net, testNode)
				if testNode.IsRunning() {
					defer testNode.Stop() // Clean up if node was started
				}

				msg := NewMessage("sender", "receiver", "test")
				net.Send("sender", "receiver", msg)

				time.Sleep(50 * time.Millisecond)
				synctest.Wait()
				net.ProcessMessages()

				time.Sleep(50 * time.Millisecond)
				synctest.Wait()

				messages := testNode.GetReceivedMessages()
				assert.Len(t, messages, tt.expectedMessages)

				if testNode.IsRunning() {
					testNode.Stop()
				}
			})
		})
	}
}
