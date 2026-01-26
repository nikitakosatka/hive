package time

import (
	"math/rand"
	stdtime "time"
)

const (
	// DefaultSynchronousLatency is the default latency used for synchronous time models.
	DefaultSynchronousLatency = 10 * stdtime.Millisecond

	// PartiallySynchronousAsyncMultiplier is the multiplier applied to latency during
	// the asynchronous phase of a partially synchronous system.
	PartiallySynchronousAsyncMultiplier = 10

	// DefaultAsynchronousLatencyRange is the maximum latency range for asynchronous systems
	// when no specific latency distribution is provided.
	DefaultAsynchronousLatencyRange = stdtime.Second
)

// Time represents time assumptions for message passing over the network.
type Time struct {
	// Model represents the timing model (Synchronous, Asynchronous, or PartiallySynchronous)
	Model Model

	// Latency defines the latency distribution for messages
	Latency LatencyDistribution

	// StabilizationTime is the time after which a partially synchronous system becomes synchronous
	StabilizationTime stdtime.Duration

	// Jitter adds random variance to message latencies to enable natural reordering.
	// Applied as a percentage of the base latency (0.0 = no jitter, 0.1 = +-10% variance).
	Jitter float64

	rng *rand.Rand
}

// Model represents the timing assumptions of the distributed system.
type Model string

const (
	// Synchronous means all messages have a known, bounded latency
	Synchronous Model = "synchronous"

	// Asynchronous means messages have no guaranteed latency bounds
	Asynchronous Model = "asynchronous"

	// PartiallySynchronous means the system eventually becomes synchronous
	// (after some unknown time, messages have bounded latency)
	PartiallySynchronous Model = "partially_synchronous"
)

// LatencyDistribution defines how message latencies are distributed.
type LatencyDistribution interface {
	// GetLatency returns the latency for a message delivery
	GetLatency() stdtime.Duration
}

// ConstantLatency always returns the same latency value.
type ConstantLatency struct {
	Latency stdtime.Duration
}

func (c *ConstantLatency) GetLatency() stdtime.Duration {
	return c.Latency
}

// UniformLatency returns a latency uniformly distributed between Min and Max.
type UniformLatency struct {
	Min stdtime.Duration
	Max stdtime.Duration
	rng *rand.Rand
}

func NewUniformLatency(min, max stdtime.Duration) *UniformLatency {
	return &UniformLatency{
		Min: min,
		Max: max,
		rng: rand.New(rand.NewSource(stdtime.Now().UnixNano())),
	}
}

func (u *UniformLatency) GetLatency() stdtime.Duration {
	if u.Max <= u.Min {
		return u.Min
	}
	diff := u.Max - u.Min
	return u.Min + stdtime.Duration(u.rng.Int63n(int64(diff)))
}

// NormalLatency returns a latency following a normal distribution.
type NormalLatency struct {
	Mean   stdtime.Duration
	StdDev stdtime.Duration
	rng    *rand.Rand
}

func NewNormalLatency(mean, stdDev stdtime.Duration) *NormalLatency {
	return &NormalLatency{
		Mean:   mean,
		StdDev: stdDev,
		rng:    rand.New(rand.NewSource(stdtime.Now().UnixNano())),
	}
}

func (n *NormalLatency) GetLatency() stdtime.Duration {
	// Use standard normal distribution
	z0 := n.rng.NormFloat64() // Standard normal

	latency := float64(n.Mean) + z0*float64(n.StdDev)
	if latency < 0 {
		latency = 0
	}
	return stdtime.Duration(latency)
}

// NewTime creates a new Time configuration.
func NewTime(model Model, latency LatencyDistribution, jitter float64) *Time {
	return &Time{
		Model:             model,
		Latency:           latency,
		Jitter:            jitter,
		StabilizationTime: 0,
		rng:               rand.New(rand.NewSource(stdtime.Now().UnixNano())),
	}
}

// NewPartiallySynchronousTime creates a new Time configuration for partially synchronous systems.
func NewPartiallySynchronousTime(latency LatencyDistribution, jitter float64, stabilizationTime stdtime.Duration) *Time {
	return &Time{
		Model:             PartiallySynchronous,
		Latency:           latency,
		Jitter:            jitter,
		StabilizationTime: stabilizationTime,
		rng:               rand.New(rand.NewSource(stdtime.Now().UnixNano())),
	}
}

// GetLatency returns the latency based on the time model configuration.
func (t *Time) GetLatency(elapsed stdtime.Duration) stdtime.Duration {
	var baseLatency stdtime.Duration

	switch t.Model {
	case Synchronous:
		if t.Latency != nil {
			baseLatency = t.Latency.GetLatency()
		} else {
			baseLatency = DefaultSynchronousLatency
		}

	case Asynchronous:
		// no bounds, but we still need some latency for simulation
		if t.Latency != nil {
			baseLatency = t.Latency.GetLatency()
		} else {
			// random between 0 and DefaultAsynchronousLatencyRange
			baseLatency = stdtime.Duration(t.rng.Int63n(int64(DefaultAsynchronousLatencyRange)))
		}

	case PartiallySynchronous:
		// Before stabilization: asynchronous behavior
		// After stabilization: synchronous behavior
		if elapsed < t.StabilizationTime {
			// Asynchronous phase
			if t.Latency != nil {
				// Use a larger variance for async phase
				baseLatency = t.Latency.GetLatency() * PartiallySynchronousAsyncMultiplier
			} else {
				baseLatency = stdtime.Duration(t.rng.Int63n(int64(DefaultAsynchronousLatencyRange)))
			}
		} else {
			// Synchronous phase
			if t.Latency != nil {
				baseLatency = t.Latency.GetLatency()
			} else {
				baseLatency = DefaultSynchronousLatency
			}
		}

	default:
		baseLatency = DefaultSynchronousLatency
	}

	// Apply jitter if specified
	return t.ApplyJitter(baseLatency)
}

// ApplyJitter adds random variance to a latency to enable natural message reordering.
func (t *Time) ApplyJitter(baseLatency stdtime.Duration) stdtime.Duration {
	if t.Jitter <= 0.0 {
		return baseLatency
	}

	jitterFactor := 1.0 + (t.rng.Float64()*2.0-1.0)*t.Jitter
	if jitterFactor < 0.0 {
		jitterFactor = 0.0 // Don't allow negative latencies
	}

	return stdtime.Duration(float64(baseLatency) * jitterFactor)
}
