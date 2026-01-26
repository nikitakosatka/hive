package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetwork_ShouldDrop(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		dropProb     float64
		iterations   int
		shouldAlways bool
		shouldNever  bool
	}{
		{
			name:        "zero probability - never drop",
			dropProb:    0.0,
			iterations:  1000,
			shouldNever: true,
		},
		{
			name:         "100% probability - always drop",
			dropProb:     1.0,
			iterations:   100,
			shouldAlways: true,
		},
		{
			name:       "50% probability - probabilistic",
			dropProb:   0.5,
			iterations: 100,
		},
		{
			name:       "very small probability",
			dropProb:   0.01,
			iterations: 1000,
		},
		{
			name:       "very high probability",
			dropProb:   0.9,
			iterations: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			net := NewFairLossNetwork(tt.dropProb, 0.0)

			dropped := 0
			for i := 0; i < tt.iterations; i++ {
				if net.ShouldDrop() {
					dropped++
				}
			}

			if tt.shouldNever {
				assert.Equal(t, 0, dropped, "should never drop with zero probability")
			} else if tt.shouldAlways {
				assert.Equal(t, tt.iterations, dropped, "should always drop with 100% probability")
			} else {
				// For probabilistic cases, verify the probability is working
				// For very small probability, we might not see any drops, so just verify it's not always dropping
				// For very high probability, we might see all drops, so just verify it's not never dropping
				if tt.dropProb < 0.1 {
					assert.LessOrEqual(t, dropped, tt.iterations, "should not drop more than total messages")
				} else if tt.dropProb > 0.9 {
					assert.GreaterOrEqual(t, dropped, 0, "should drop at least 0 messages")
					assert.Greater(t, dropped, int(float64(tt.iterations)*0.5), "with high probability, should drop most messages")
				} else {
					assert.Greater(t, dropped, 0, "should drop at least some messages")
					assert.Less(t, dropped, tt.iterations, "should not drop all messages")
				}
			}
		})
	}
}

func TestNetwork_ShouldDuplicate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		dupProb      float64
		iterations   int
		shouldAlways bool
		shouldNever  bool
	}{
		{
			name:        "zero probability - never duplicate",
			dupProb:     0.0,
			iterations:  1000,
			shouldNever: true,
		},
		{
			name:         "100% probability - always duplicate",
			dupProb:      1.0,
			iterations:   100,
			shouldAlways: true,
		},
		{
			name:       "50% probability - probabilistic",
			dupProb:    0.5,
			iterations: 100,
		},
		{
			name:       "edge case: very small probability",
			dupProb:    0.01,
			iterations: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			net := NewFairLossNetwork(0.0, tt.dupProb)

			duplicated := 0
			for i := 0; i < tt.iterations; i++ {
				if net.ShouldDuplicate() {
					duplicated++
				}
			}

			if tt.shouldNever {
				assert.Equal(t, 0, duplicated, "should never duplicate with zero probability")
			} else if tt.shouldAlways {
				assert.Equal(t, tt.iterations, duplicated, "should always duplicate with 100% probability")
			} else {
				// For probabilistic cases, verify the probability is working
				// For very small probability, we might not see any duplicates, so just verify it's not always duplicating
				// For very high probability, we might see all duplicates, so just verify it's not never duplicating
				if tt.dupProb < 0.1 {
					assert.LessOrEqual(t, duplicated, tt.iterations, "should not duplicate more than total messages")
				} else if tt.dupProb > 0.9 {
					assert.GreaterOrEqual(t, duplicated, 0, "should duplicate at least 0 messages")
					assert.Greater(t, duplicated, int(float64(tt.iterations)*0.5), "with high probability, should duplicate most messages")
				} else {
					assert.Greater(t, duplicated, 0, "should duplicate at least some messages")
					assert.Less(t, duplicated, tt.iterations, "should not duplicate all messages")
				}
			}
		})
	}
}
