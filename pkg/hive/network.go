package hive

import (
	"sync"
	"time"

	"github.com/nikitakosatka/hive/pkg/failure"
	"github.com/nikitakosatka/hive/pkg/network"
	timemodel "github.com/nikitakosatka/hive/pkg/time"
)

const (
	// DuplicateMessageDelay is the delay added to duplicate messages to ensure they arrive after the original.
	DuplicateMessageDelay = 1 * time.Millisecond
)

// Network handles message routing between nodes.
type Network struct {
	networkConfig *network.Network
	timeConfig    *timemodel.Time
	failConfig    *failure.Failure
	nodes         map[string]Node
	mu            sync.RWMutex

	// pendingMessages holds messages waiting to be routed
	pendingMessages []*pendingMessage
	pmMu            sync.Mutex

	// startTime tracks when the network started
	startTime time.Time
}

type pendingMessage struct {
	msg       *Message
	deliverAt time.Time
}

// NewNetwork creates a new network with the given configuration.
func NewNetwork(networkConfig *network.Network, timeConfig *timemodel.Time, failConfig *failure.Failure) *Network {
	return &Network{
		networkConfig:   networkConfig,
		timeConfig:      timeConfig,
		failConfig:      failConfig,
		nodes:           make(map[string]Node),
		pendingMessages: make([]*pendingMessage, 0),
		startTime:       time.Now(),
	}
}

// RegisterNode registers a node with the network.
func (n *Network) RegisterNode(node Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.nodes[node.ID()] = node
}

// UnregisterNode removes a node from the network.
func (n *Network) UnregisterNode(nodeID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.nodes, nodeID)
}

// Send attempts to send a message from one node to another.
// If the receiver is crashed, the message is silently dropped (no error).
func (n *Network) Send(fromID, toID string, msg *Message) error {
	// Check if sender is crashed
	if n.failConfig.IsNodeCrashed(fromID) {
		return ErrNodeNotRunning
	}

	// Check if receiver is crashed - silently drop message (no error)
	if n.failConfig.IsNodeCrashed(toID) {
		return nil // Message dropped, no error
	}

	// Check if message should be dropped
	if n.networkConfig.ShouldDrop() {
		return nil // Message dropped, no error
	}

	// Calculate delivery time based on time model
	elapsed := time.Since(n.startTime)
	latency := n.timeConfig.GetLatency(elapsed)
	deliverAt := time.Now().Add(latency)

	// Create pending message
	pm := &pendingMessage{
		msg:       msg,
		deliverAt: deliverAt,
	}

	// Handle duplication
	n.pmMu.Lock()
	n.pendingMessages = append(n.pendingMessages, pm)

	if n.networkConfig.ShouldDuplicate() {
		// Add duplicate with slightly later delivery time
		duplicate := &pendingMessage{
			msg:       msg,
			deliverAt: deliverAt.Add(DuplicateMessageDelay),
		}
		n.pendingMessages = append(n.pendingMessages, duplicate)
	}
	n.pmMu.Unlock()

	return nil
}

// ProcessMessages processes and routes pending messages to their destination nodes.
// This should be called periodically by the simulator.
func (n *Network) ProcessMessages() {
	n.pmMu.Lock()
	now := time.Now()
	toDeliver := make([]*pendingMessage, 0)
	remaining := make([]*pendingMessage, 0)

	for _, pm := range n.pendingMessages {
		if now.After(pm.deliverAt) || now.Equal(pm.deliverAt) {
			toDeliver = append(toDeliver, pm)
		} else {
			remaining = append(remaining, pm)
		}
	}

	n.pendingMessages = remaining
	n.pmMu.Unlock()

	// Route messages to nodes
	for _, pm := range toDeliver {
		// Check if receiver is still running and not crashed
		n.mu.RLock()
		node, exists := n.nodes[pm.msg.To]
		n.mu.RUnlock()

		if !exists {
			continue // Node doesn't exist
		}

		if !node.IsRunning() {
			continue // Node stopped
		}

		if n.failConfig.IsNodeCrashed(pm.msg.To) {
			continue // Node crashed, message dropped
		}

		// Route message to node - try queued delivery first, fallback to direct call
		if queueNode, ok := node.(MessageQueueNode); ok {
			// Node supports queued message delivery (e.g., BaseNode or nodes embedding it)
			if err := queueNode.EnqueueMessage(pm.msg); err != nil {
				// If enqueue fails, fall back to direct Receive call
				// This handles cases where queue is full or node is stopping
				go func(msg *Message) {
					node.Receive(msg)
				}(pm.msg)
			}
		} else {
			// For custom node implementations without message queue
			go func(msg *Message) {
				node.Receive(msg)
			}(pm.msg)
		}
	}
}

// GetPendingMessageCount returns the number of messages waiting to be delivered.
func (n *Network) GetPendingMessageCount() int {
	n.pmMu.Lock()
	defer n.pmMu.Unlock()
	return len(n.pendingMessages)
}

// SetStartTime updates the network's start time.
// This is called by the simulator when it starts.
func (n *Network) SetStartTime(t time.Time) {
	n.pmMu.Lock()
	defer n.pmMu.Unlock()
	n.startTime = t
}
