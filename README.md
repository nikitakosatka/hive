# Hosted Interaction Virtual Environment

Hive is a distributed systems framework for building and simulating distributed systems in Go. It provides a single-process simulation environment where nodes communicate via in-memory channels.

## Features

- **Node Management**: Create and manage multiple nodes in a distributed system
- **Message Passing**: Send messages between nodes with configurable delivery semantics
- **Network Models**: Choose between reliable and unreliable channels
- **Time Models**: Support for Synchronous, Asynchronous, and Partially Synchronous systems
- **Failure Models**: Simulate Reliable, Crash-Stop, and Crash-Recovery scenarios
- **Configurable Latency**: Support for constant, uniform, and normal latency distributions

## Quick Start

### Installation

```bash
go get github.com/nikitakosatka/hive
```

### Basic Example

```go
package main

import (
    "log"
    "time"

    "github.com/nikitakosatka/hive/pkg/failure"
    "github.com/nikitakosatka/hive/pkg/hive"
    "github.com/nikitakosatka/hive/pkg/network"
    timemodel "github.com/nikitakosatka/hive/pkg/time"
)

// Create a simple node
type MyNode struct {
    *hive.BaseNode
}

func (n *MyNode) Receive(msg *hive.Message) error {
    log.Printf("[%s] Received message from %s: %v", n.ID(), msg.From, msg.Payload)
    return nil
}

func main() {
    // Configure the simulator
    config := hive.NewConfig(
        hive.WithNetwork(network.NewReliableNetwork()),
        hive.WithTime(timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 10 * time.Millisecond}, 0.0)),
        hive.WithNodesFailures(failure.NewCrashStop(0.0)), // No crashes
    )
    
    sim := hive.NewSimulator(config)
    
    // Create and add nodes
    node1 := &MyNode{BaseNode: hive.NewBaseNode("node1")}
    node2 := &MyNode{BaseNode: hive.NewBaseNode("node2")}
    
    sim.AddNode(node1)
    sim.AddNode(node2)
    
    // Start the simulator
    sim.Start()
    defer sim.Stop()
    
    // Send a message
    node1.Send("node2", "Hello!")
    
    // Wait for message delivery
    time.Sleep(100 * time.Millisecond)
}
```

## Configuration Options

The framework provides three main configuration categories: Network, Time, and Node Failures.

### Network Configuration

The network configuration controls message delivery reliability:

```go
import (
	"github.com/nikitakosatka/hive/pkg/hive"
	"github.com/nikitakosatka/hive/pkg/network"
)

// Reliable network: no drops, no duplicates
network := network.NewReliableNetwork()
config := hive.NewConfig(
    hive.WithNetwork(network),
)

// Fair-loss network: configurable drop and duplicate probabilities
network := network.NewFairLossNetwork(
    0.1,  // Drop probability (10%)
    0.05, // Duplicate probability (5%)
)
config := hive.NewConfig(
    hive.WithNetwork(network),
)
```

### Time Configuration

The time configuration defines timing assumptions and latency distributions:

```go
import (
	"github.com/nikitakosatka/hive/pkg/hive"
	timemodel "github.com/nikitakosatka/hive/pkg/time"
)

// Synchronous system with constant latency
time := timemodel.NewTime(
    timemodel.Synchronous,
    &timemodel.ConstantLatency{Latency: 10 * time.Millisecond},
    0.0, // No jitter
)

// Asynchronous system with uniform latency distribution
time := timemodel.NewTime(
    timemodel.Asynchronous,
    timemodel.NewUniformLatency(5*time.Millisecond, 20*time.Millisecond),
    0.1, // 10% jitter
)

// Partially synchronous system (eventually becomes synchronous)
time := timemodel.NewPartiallySynchronousTime(
    &timemodel.ConstantLatency{Latency: 10 * time.Millisecond},
    0.1,                        // 10% jitter
    5*time.Second,              // Stabilization time
)

// Normal (Gaussian) latency distribution
time := timemodel.NewTime(
    timemodel.Synchronous,
    timemodel.NewNormalLatency(10*time.Millisecond, 2*time.Millisecond),
    0.0,
)

config := hive.NewConfig(
    hive.WithTime(time),
)
```

**Time Models:**
- `timemodel.Synchronous`: All messages have bounded, known latency
- `timemodel.Asynchronous`: No latency guarantees
- `timemodel.PartiallySynchronous`: Eventually becomes synchronous after stabilization time

**Latency Distributions:**
- `timemodel.ConstantLatency`: Fixed latency value
- `timemodel.UniformLatency`: Uniform distribution between min and max
- `timemodel.NormalLatency`: Normal (Gaussian) distribution with mean and standard deviation

### Node Failure Configuration

The failure configuration controls node crash and recovery behavior:

```go
import (
	"github.com/nikitakosatka/hive/pkg/hive"
	"github.com/nikitakosatka/hive/pkg/failure"
)

// Reliable: nodes never crash
failure := failure.NewCrashStop(0.0) // Zero crash probability

// Crash-stop: nodes can crash but never recover
failure := failure.NewCrashStop(0.01) // 1% crash probability

// Crash-recovery: nodes can crash and later recover
failure := failure.NewCrashRecovery(
    0.01,                  // Crash probability (1%)
    5*time.Second,         // Recovery after 5 seconds
    0.2,                   // 20% jitter on recovery time
)

config := hive.NewConfig(
    hive.WithNodesFailures(failure),
)
```

**Note:** When sending a message to a crashed node, the message is silently dropped (no error is returned).

## Implementing Logical Clocks

The framework is designed to make it easy to implement logical clocks (Lamport, vector clocks, etc.) by wrapping the `Send` and `Receive` methods.

Key points:
- Use `Message.Metadata` to store logical clock values
- Override `Send()` to add clock timestamps before sending
- Override `Receive()` to update clocks when receiving messages
- Use `SendMessage()` to send pre-constructed messages with custom metadata

Example snippet:

```go
func (n *LamportClockNode) Send(to string, payload interface{}) error {
    clock := n.tick()
    msg := hive.NewMessage(n.ID(), to, payload)
    msg.Metadata["lamport_clock"] = clock
    return n.SendMessage(msg)
}

func (n *LamportClockNode) Receive(msg *hive.Message) error {
    if receivedClock, ok := msg.Metadata["lamport_clock"].(int64); ok {
        n.updateClock(receivedClock)
    }
    // Process message...
    return nil
}
```

## Examples

### Ping-Pong Example

Demonstrates basic message passing between nodes:

```bash
cd examples/ping_pong
go run main.go
```

## Message Flow

1. Node calls `Send(to, payload)`
2. Network layer processes the message (applies latency, may drop/duplicate)
3. Message is routed to target node's `Receive()`
4. Node processes the message

## Extending the Framework

### Creating Custom Nodes

Embed `BaseNode` and override `Receive()`:

```go
type CustomNode struct {
    *hive.BaseNode
    // Your custom fields
}

func (n *CustomNode) Receive(msg *hive.Message) error {
    // Your message handling logic
    return nil
}
```

### Adding Custom Metadata

Use `Message.Metadata` to add custom fields:

```go
msg := hive.NewMessage("node1", "node2", payload)
msg.Metadata["custom_field"] = value
```

 [MIT License](./LICENSE)