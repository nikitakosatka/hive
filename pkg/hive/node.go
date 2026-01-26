package hive

import (
	"context"
	"sync"
)

const (
	// DefaultMessageQueueBufferSize is the default buffer size for node message queues.
	DefaultMessageQueueBufferSize = 100
)

// Node represents a node in the distributed system.
// Students should implement this interface to create their own nodes.
type Node interface {
	// ID returns the unique identifier of this node
	ID() string

	// Start initializes the node and begins processing messages
	Start(ctx context.Context) error

	// Stop gracefully shuts down the node
	Stop() error

	// Receive processes an incoming message
	// This is called by the network layer when a message arrives
	Receive(msg *Message) error

	// Send is called by the node to send a message to another node
	// The actual delivery is handled by the network layer
	Send(to string, payload interface{}) error

	// IsRunning returns whether the node is currently running
	IsRunning() bool
}

// MessageQueueNode is a node that supports queued message delivery.
// BaseNode implements this interface. Nodes embedding BaseNode will also implement it.
type MessageQueueNode interface {
	Node
	// EnqueueMessage adds a message to the node's message queue.
	// This is called by the network layer.
	EnqueueMessage(msg *Message) error
}

// SendConfigurableNode is a node that can have its send function configured.
// BaseNode implements this interface. Nodes embedding BaseNode will also implement it.
type SendConfigurableNode interface {
	Node
	// SetSendFunc sets the function that will be called when Send() is invoked.
	SetSendFunc(fn func(string, *Message) error)
	// SetNodeRef sets the Node interface reference for proper method dispatch.
	SetNodeRef(node Node)
}

// BaseNode provides a basic implementation of Node that students can embed.
// It handles message queuing and basic lifecycle management.
type BaseNode struct {
	id      string
	running bool
	mu      sync.RWMutex

	// messageQueue holds incoming messages
	messageQueue chan *Message

	// sendFunc is called when Send() is invoked
	sendFunc func(string, *Message) error

	// nodeRef is a reference to this node as a Node interface
	// This allows processMessages to call the overridden Receive method
	nodeRef Node

	// ctx and cancel for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewBaseNode creates a new BaseNode with the given ID.
func NewBaseNode(id string) *BaseNode {
	return &BaseNode{
		id:           id,
		messageQueue: make(chan *Message, DefaultMessageQueueBufferSize),
	}
}

// ID returns the node's identifier.
func (n *BaseNode) ID() string {
	return n.id
}

// Start initializes the node and starts processing messages.
func (n *BaseNode) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.running {
		return nil // Already running
	}

	// Recreate message queue if it was closed or nil
	if n.messageQueue == nil {
		n.messageQueue = make(chan *Message, DefaultMessageQueueBufferSize)
	}

	n.ctx, n.cancel = context.WithCancel(ctx)
	n.running = true

	// Start message processing goroutine
	n.wg.Add(1)
	go n.processMessages()

	return nil
}

// Stop gracefully shuts down the node.
func (n *BaseNode) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	n.running = false
	if n.cancel != nil {
		n.cancel()
	}

	// Don't close the channel - we may want to restart the node
	// The processMessages goroutine will exit when ctx is cancelled
	n.wg.Wait()

	return nil
}

// processMessages continuously processes incoming messages.
func (n *BaseNode) processMessages() {
	defer n.wg.Done()

	for {
		select {
		case <-n.ctx.Done():
			return
		case msg, ok := <-n.messageQueue:
			if !ok {
				return
			}
			// Call Receive through nodeRef to ensure overridden methods are called
			if n.nodeRef != nil {
				n.nodeRef.Receive(msg)
			} else {
				// Fallback to direct call if nodeRef not set
				n.Receive(msg)
			}
		}
	}
}

// Receive processes an incoming message.
// Students should override this method in their node implementations.
func (n *BaseNode) Receive(msg *Message) error {
	// Default implementation does nothing
	// Students should override this
	return nil
}

// Send sends a message to another node.
// Students can wrap this to add logical clock logic.
func (n *BaseNode) Send(to string, payload interface{}) error {
	n.mu.RLock()
	if !n.running {
		n.mu.RUnlock()
		return ErrNodeNotRunning
	}
	n.mu.RUnlock()

	msg := NewMessage(n.id, to, payload)

	if n.sendFunc != nil {
		return n.sendFunc(to, msg)
	}

	return ErrNoSendFunction
}

// SetSendFunc sets the function that will be called when Send() is invoked.
// This is typically set by the Simulator.
func (n *BaseNode) SetSendFunc(fn func(string, *Message) error) {
	n.sendFunc = fn
}

// SetNodeRef sets the Node interface reference for this BaseNode.
// This allows processMessages to call overridden Receive methods.
// This is typically set by the Simulator.
func (n *BaseNode) SetNodeRef(node Node) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.nodeRef = node
}

// SendMessage sends a pre-constructed message.
// This allows students to wrap Send() and add custom metadata (e.g., logical clocks).
func (n *BaseNode) SendMessage(msg *Message) error {
	n.mu.RLock()
	if !n.running {
		n.mu.RUnlock()
		return ErrNodeNotRunning
	}
	n.mu.RUnlock()

	if n.sendFunc != nil {
		return n.sendFunc(msg.To, msg)
	}

	return ErrNoSendFunction
}

// IsRunning returns whether the node is currently running.
func (n *BaseNode) IsRunning() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.running
}

// EnqueueMessage adds a message to the node's message queue.
// This is called by the network layer.
func (n *BaseNode) EnqueueMessage(msg *Message) error {
	n.mu.RLock()
	if !n.running {
		n.mu.RUnlock()
		return ErrNodeNotRunning
	}
	ctx := n.ctx
	n.mu.RUnlock()

	select {
	case n.messageQueue <- msg:
		return nil
	case <-ctx.Done():
		return ErrNodeStopped
	default:
		return ErrMessageQueueFull
	}
}

// Context returns the node's context for cancellation.
// Students can use this to create goroutines that respect node lifecycle.
func (n *BaseNode) Context() context.Context {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.ctx
}
