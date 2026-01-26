package main

import (
	"log"
	"time"

	"github.com/nikitakosatka/hive/pkg/failure"
	"github.com/nikitakosatka/hive/pkg/hive"
	"github.com/nikitakosatka/hive/pkg/network"
	timemodel "github.com/nikitakosatka/hive/pkg/time"
)

// PingPongNode is a simple node that sends ping messages and responds to pong messages.
type PingPongNode struct {
	*hive.BaseNode
	pingCount int
	pongCount int
	logger    *log.Logger
}

// logf logs a formatted message with the node ID prefix.
func (n *PingPongNode) logf(format string, v ...interface{}) {
	if n.logger == nil {
		n.logger = log.New(log.Writer(), "", log.LstdFlags)
	}
	n.logger.Printf("[%s] "+format, append([]interface{}{n.ID()}, v...)...)
}

// NewPingPongNode creates a new ping-pong node.
func NewPingPongNode(id string) *PingPongNode {
	return &PingPongNode{
		BaseNode:  hive.NewBaseNode(id),
		pingCount: 0,
		pongCount: 0,
	}
}

// Receive processes incoming messages.
func (n *PingPongNode) Receive(msg *hive.Message) error {
	switch payload := msg.Payload.(type) {
	case string:
		if payload == "ping" {
			n.logf("Received PING from %s", msg.From)
			// Respond with pong
			n.Send(msg.From, "pong")
		} else if payload == "pong" {
			n.pongCount++
			n.logf("Received PONG from %s (total pongs: %d)", msg.From, n.pongCount)
		}
	}
	return nil
}

// StartPinging starts sending periodic ping messages to the target node.
func (n *PingPongNode) StartPinging(targetID string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		ctx := n.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n.IsRunning() {
					n.pingCount++
					n.logf("Sending PING #%d to %s", n.pingCount, targetID)
					n.Send(targetID, "ping")
				}
			}
		}
	}()
}

func main() {
	// Create simulator with reliable network and synchronous time model
	config := hive.NewConfig(
		hive.WithNetwork(network.NewReliableNetwork()),
		hive.WithTime(timemodel.NewTime(timemodel.Synchronous, &timemodel.ConstantLatency{Latency: 50 * time.Millisecond}, 0.0)),
		hive.WithNodesFailures(failure.NewCrashStop(0.0)), // No crashes
	)

	sim := hive.NewSimulator(config)

	// Create two nodes
	node1 := NewPingPongNode("node1")
	node2 := NewPingPongNode("node2")

	// Add nodes to simulator
	if err := sim.AddNode(node1); err != nil {
		log.Fatal(err)
	}
	if err := sim.AddNode(node2); err != nil {
		log.Fatal(err)
	}

	// Start simulator
	if err := sim.Start(); err != nil {
		log.Fatal(err)
	}

	// Start ping-pong
	node1.StartPinging("node2", 1*time.Second)

	// Run for 10 seconds
	log.Println("Running ping-pong simulation for 10 seconds...")
	time.Sleep(10 * time.Second)
	sim.Stop()

	log.Println("\nFinal stats:")
	log.Printf("Node1 sent %d pings, received %d pongs", node1.pingCount, node1.pongCount)
	log.Printf("Node2 sent %d pings, received %d pongs", node2.pingCount, node2.pongCount)
}
