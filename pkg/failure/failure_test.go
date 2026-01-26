package failure

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFailure_IsNodeCrashed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func(*Failure)
		nodeID   string
		expected bool
	}{
		{
			name:     "node not crashed",
			setup:    func(f *Failure) {},
			nodeID:   "node1",
			expected: false,
		},
		{
			name: "node crashed",
			setup: func(f *Failure) {
				f.CrashNode("node1")
			},
			nodeID:   "node1",
			expected: true,
		},
		{
			name: "different node not crashed",
			setup: func(f *Failure) {
				f.CrashNode("node1")
			},
			nodeID:   "node2",
			expected: false,
		},
		{
			name: "multiple nodes crashed",
			setup: func(f *Failure) {
				f.CrashNode("node1")
				f.CrashNode("node2")
			},
			nodeID:   "node1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewCrashStop(0.0)
			tt.setup(f)

			result := f.IsNodeCrashed(tt.nodeID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFailure_CrashNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		nodesToCrash []string
		checkNode    string
		expected     bool
	}{
		{
			name:         "single node crash",
			nodesToCrash: []string{"node1"},
			checkNode:    "node1",
			expected:     true,
		},
		{
			name:         "multiple nodes crash",
			nodesToCrash: []string{"node1", "node2", "node3"},
			checkNode:    "node2",
			expected:     true,
		},
		{
			name:         "crash same node twice",
			nodesToCrash: []string{"node1", "node1"},
			checkNode:    "node1",
			expected:     true,
		},
		{
			name:         "edge case: empty node ID",
			nodesToCrash: []string{""},
			checkNode:    "",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewCrashStop(0.0)

			for _, nodeID := range tt.nodesToCrash {
				f.CrashNode(nodeID)
			}

			assert.Equal(t, tt.expected, f.IsNodeCrashed(tt.checkNode))
		})
	}
}

func TestFailure_RecoverNode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setup     func(*Failure)
		recover   string
		checkNode string
		expected  bool
	}{
		{
			name: "recover crashed node",
			setup: func(f *Failure) {
				f.CrashNode("node1")
			},
			recover:   "node1",
			checkNode: "node1",
			expected:  false,
		},
		{
			name: "recover non-crashed node",
			setup: func(f *Failure) {
				// Don't crash anything
			},
			recover:   "node1",
			checkNode: "node1",
			expected:  false,
		},
		{
			name: "recover one of multiple crashed nodes",
			setup: func(f *Failure) {
				f.CrashNode("node1")
				f.CrashNode("node2")
			},
			recover:   "node1",
			checkNode: "node2",
			expected:  true,
		},
		{
			name: "recover and crash again",
			setup: func(f *Failure) {
				f.CrashNode("node1")
				f.RecoverNode("node1")
				f.CrashNode("node1")
			},
			recover:   "node1",
			checkNode: "node1",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewCrashRecovery(0.0, 1*time.Second, 0.0)
			tt.setup(f)

			f.RecoverNode(tt.recover)
			assert.Equal(t, tt.expected, f.IsNodeCrashed(tt.checkNode))
		})
	}
}

func TestFailure_ShouldCrash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		crashProb    float64
		setup        func(*Failure)
		nodeID       string
		iterations   int
		shouldNever  bool
		shouldAlways bool
	}{
		{
			name:        "zero probability - never crash",
			crashProb:   0.0,
			setup:       func(f *Failure) {},
			nodeID:      "node1",
			iterations:  1000,
			shouldNever: true,
		},
		{
			name:         "100% probability - always crash",
			crashProb:    1.0,
			setup:        func(f *Failure) {},
			nodeID:       "node1",
			iterations:   100,
			shouldAlways: true,
		},
		{
			name:        "already crashed - never crash again",
			crashProb:   1.0,
			setup:       func(f *Failure) { f.CrashNode("node1") },
			nodeID:      "node1",
			iterations:  100,
			shouldNever: true,
		},
		{
			name:       "probabilistic crash",
			crashProb:  0.5,
			setup:      func(f *Failure) {},
			nodeID:     "node1",
			iterations: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewCrashStop(tt.crashProb)
			tt.setup(f)

			crashed := 0
			for i := 0; i < tt.iterations; i++ {
				if f.ShouldCrash(tt.nodeID) {
					crashed++
				}
			}

			if tt.shouldNever {
				assert.Equal(t, 0, crashed)
			} else if tt.shouldAlways {
				assert.Greater(t, crashed, 0)
			}
		})
	}
}

func TestFailure_ShouldRecover(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		recoveryAfter time.Duration
		jitter        float64
		setup         func(*Failure)
		waitTime      time.Duration
		nodeID        string
		expected      bool
	}{
		{
			name:          "crash-stop - never recover",
			recoveryAfter: 0,
			jitter:        0.0,
			setup:         func(f *Failure) { f.CrashNode("node1") },
			waitTime:      0, // No need to wait for crash-stop - check immediately
			nodeID:        "node1",
			expected:      false,
		},
		{
			name:          "not crashed - cannot recover",
			recoveryAfter: 1 * time.Second,
			jitter:        0.0,
			setup:         func(f *Failure) {},
			waitTime:      0,
			nodeID:        "node1",
			expected:      false,
		},
		{
			name:          "not enough time - cannot recover",
			recoveryAfter: 1 * time.Second,
			jitter:        0.0,
			setup:         func(f *Failure) { f.CrashNode("node1") },
			waitTime:      0,
			nodeID:        "node1",
			expected:      false,
		},
		{
			name:          "enough time - can recover",
			recoveryAfter: 10 * time.Millisecond,
			jitter:        0.0,
			setup:         func(f *Failure) { f.CrashNode("node1") },
			waitTime:      20 * time.Millisecond,
			nodeID:        "node1",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(
				t, func(t *testing.T) {
					f := NewCrashRecovery(0.0, tt.recoveryAfter, tt.jitter)
					tt.setup(f)

					if tt.waitTime > 0 {
						time.Sleep(tt.waitTime)
					}

					result := f.ShouldRecover(tt.nodeID)
					assert.Equal(t, tt.expected, result)
				},
			)
		})
	}
}

func TestFailure_GetCrashedNodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		setup        func(*Failure)
		expectedLen  int
		expectedKeys []string
	}{
		{
			name:        "no crashed nodes",
			setup:       func(f *Failure) {},
			expectedLen: 0,
		},
		{
			name: "single crashed node",
			setup: func(f *Failure) {
				f.CrashNode("node1")
			},
			expectedLen:  1,
			expectedKeys: []string{"node1"},
		},
		{
			name: "multiple crashed nodes",
			setup: func(f *Failure) {
				f.CrashNode("node1")
				f.CrashNode("node2")
				f.CrashNode("node3")
			},
			expectedLen:  3,
			expectedKeys: []string{"node1", "node2", "node3"},
		},
		{
			name: "crash and recover",
			setup: func(f *Failure) {
				f.CrashNode("node1")
				f.CrashNode("node2")
				f.RecoverNode("node1")
			},
			expectedLen:  1,
			expectedKeys: []string{"node2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewCrashStop(0.0)
			tt.setup(f)

			crashed := f.GetCrashedNodes()
			assert.Len(t, crashed, tt.expectedLen)

			for _, key := range tt.expectedKeys {
				assert.True(t, crashed[key], "expected node %s to be crashed", key)
			}
		})
	}
}
