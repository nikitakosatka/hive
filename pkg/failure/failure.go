package failure

import (
	"math/rand"
	"sync"
	"time"
)

// Failure represents node failure configuration.
type Failure struct {
	// CrashProbability is the probability that a node will crash (per time unit, 0.0 to 1.0)
	CrashProbability float64

	// RecoveryAfter is the base time after which a crashed node will recover.
	// If zero, nodes will never recover (crash-stop model).
	RecoveryAfter time.Duration

	// RecoveryAfterJitter adds random variance to RecoveryAfter.
	// Applied as a percentage (0.0 = no jitter, 0.1 = +-10% variance).
	RecoveryAfterJitter float64

	// crashedNodes tracks which nodes are currently crashed
	crashedNodes map[string]bool
	crashTimes   map[string]time.Time // Track when each node crashed
	mu           sync.RWMutex
	rng          *rand.Rand
}

// NewCrashStop creates a crash-stop failure model.
// Nodes can crash but will never recover.
func NewCrashStop(crashProbability float64) *Failure {
	return &Failure{
		CrashProbability:    crashProbability,
		RecoveryAfter:       0, // Zero means no recovery
		RecoveryAfterJitter: 0.0,
		crashedNodes:        make(map[string]bool),
		crashTimes:          make(map[string]time.Time),
		rng:                 rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewCrashRecovery creates a crash-recovery failure model.
// Nodes can crash and will recover after RecoveryAfter time (with jitter).
func NewCrashRecovery(crashProbability float64, recoveryAfter time.Duration, recoveryAfterJitter float64) *Failure {
	return &Failure{
		CrashProbability:    crashProbability,
		RecoveryAfter:       recoveryAfter,
		RecoveryAfterJitter: recoveryAfterJitter,
		crashedNodes:        make(map[string]bool),
		crashTimes:          make(map[string]time.Time),
		rng:                 rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// IsNodeCrashed returns whether the given node is currently crashed.
func (f *Failure) IsNodeCrashed(nodeID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.crashedNodes[nodeID]
}

// CrashNode marks a node as crashed.
func (f *Failure) CrashNode(nodeID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.crashedNodes[nodeID] = true
	f.crashTimes[nodeID] = time.Now()
}

// RecoverNode marks a node as recovered.
func (f *Failure) RecoverNode(nodeID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.crashedNodes, nodeID)
	delete(f.crashTimes, nodeID)
}

// ShouldCrash determines if a node should crash based on the failure model.
func (f *Failure) ShouldCrash(nodeID string) bool {
	if f.CrashProbability <= 0.0 {
		return false
	}

	if f.IsNodeCrashed(nodeID) {
		return false // Already crashed
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rng.Float64() < f.CrashProbability
}

// ShouldRecover determines if a crashed node should recover.
func (f *Failure) ShouldRecover(nodeID string) bool {
	if f.RecoveryAfter <= 0 {
		return false // No recovery (crash-stop)
	}

	f.mu.RLock()
	crashed := f.crashedNodes[nodeID]
	if !crashed {
		f.mu.RUnlock()
		return false
	}
	crashTime := f.crashTimes[nodeID]
	f.mu.RUnlock()

	// Calculate recovery time with jitter
	recoveryTime := f.RecoveryAfter
	if f.RecoveryAfterJitter > 0.0 {
		f.mu.Lock()
		jitterFactor := 1.0 + (f.rng.Float64()*2.0-1.0)*f.RecoveryAfterJitter
		if jitterFactor < 0.0 {
			jitterFactor = 0.0
		}
		recoveryTime = time.Duration(float64(f.RecoveryAfter) * jitterFactor)
		f.mu.Unlock()
	}

	// Check if enough time has passed since crash
	elapsed := time.Since(crashTime)
	return elapsed >= recoveryTime
}

// GetCrashedNodes returns a copy of the set of crashed node IDs.
func (f *Failure) GetCrashedNodes() map[string]bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]bool)
	for id := range f.crashedNodes {
		result[id] = true
	}
	return result
}
