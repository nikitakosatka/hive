package network

import (
	"math/rand"
	"sync"
	"time"
)

// Network represents network configuration with drop and duplicate probabilities.
type Network struct {
	// DropProbability is the probability that a message will be dropped (0.0 to 1.0)
	DropProbability float64

	// DuplicateProbability is the probability that a message will be duplicated (0.0 to 1.0)
	DuplicateProbability float64

	rng *rand.Rand
	mu  sync.Mutex
}

// NewReliableNetwork creates a reliable network with zero drop and duplicate probabilities.
func NewReliableNetwork() *Network {
	return &Network{
		DropProbability:      0.0,
		DuplicateProbability: 0.0,
		rng:                  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewFairLossNetwork creates a fair-loss network with specified drop and duplicate probabilities.
func NewFairLossNetwork(dropProbability, duplicateProbability float64) *Network {
	return &Network{
		DropProbability:      dropProbability,
		DuplicateProbability: duplicateProbability,
		rng:                  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ShouldDrop determines if a message should be dropped.
func (n *Network) ShouldDrop() bool {
	if n.DropProbability <= 0.0 {
		return false
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	return n.rng.Float64() < n.DropProbability
}

// ShouldDuplicate determines if a message should be duplicated.
func (n *Network) ShouldDuplicate() bool {
	if n.DuplicateProbability <= 0.0 {
		return false
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	return n.rng.Float64() < n.DuplicateProbability
}
