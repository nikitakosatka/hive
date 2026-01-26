package hive

import (
	"github.com/nikitakosatka/hive/pkg/failure"
	"github.com/nikitakosatka/hive/pkg/network"
	timemodel "github.com/nikitakosatka/hive/pkg/time"
)

// Config holds all configuration options for the distributed system simulator.
type Config struct {
	// Network configuration
	Network *network.Network

	// Time configuration
	Time *timemodel.Time

	// Failure configuration
	Failure *failure.Failure
}

// Option is a function that modifies the Config.
type Option func(*Config)

// WithNetwork sets the network configuration.
func WithNetwork(n *network.Network) Option {
	return func(c *Config) {
		c.Network = n
	}
}

// WithNodesFailures sets the node failure configuration.
func WithNodesFailures(f *failure.Failure) Option {
	return func(c *Config) {
		c.Failure = f
	}
}

// WithTime sets the time configuration.
func WithTime(t *timemodel.Time) Option {
	return func(c *Config) {
		c.Time = t
	}
}

// NewConfig creates a new Config with default values.
func NewConfig(opts ...Option) *Config {
	config := &Config{
		Network: network.NewReliableNetwork(),
		Time:    timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: timemodel.DefaultSynchronousLatency}, 0.0),
		Failure: failure.NewCrashStop(0.0), // No crashes by default
	}

	for _, opt := range opts {
		opt(config)
	}

	return config
}
