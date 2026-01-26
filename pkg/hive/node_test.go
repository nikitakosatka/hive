package hive

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNode struct {
	*BaseNode
	receivedMessages []*Message
	mu               sync.Mutex
	messageReceived  chan struct{}
}

func newTestNode(id string) *testNode {
	return &testNode{
		BaseNode:         NewBaseNode(id),
		receivedMessages: make([]*Message, 0),
		messageReceived:  make(chan struct{}, 100), // Buffer for multiple messages
	}
}

func (n *testNode) Receive(msg *Message) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.receivedMessages = append(n.receivedMessages, msg)
	select {
	case n.messageReceived <- struct{}{}:
	default:
	}
	return nil
}

func (n *testNode) GetReceivedMessages() []*Message {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]*Message, len(n.receivedMessages))
	copy(result, n.receivedMessages)
	return result
}

func (n *testNode) waitForMessages(count int, timeoutCh <-chan struct{}) bool {
	received := 0
	for received < count {
		select {
		case <-n.messageReceived:
			received++
		case <-timeoutCh:
			return false
		}
	}
	return true
}

func TestBaseNode_Send(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*BaseNode, context.Context)
		expectError bool
		errorType   error
	}{
		{
			name: "no send func - error",
			setup: func(n *BaseNode, ctx context.Context) {
				n.Start(ctx)
			},
			expectError: true,
			errorType:   ErrNoSendFunction,
		},
		{
			name: "not running - error",
			setup: func(n *BaseNode, ctx context.Context) {
				// Don't start
			},
			expectError: true,
			errorType:   ErrNodeNotRunning,
		},
		{
			name: "with send func - success",
			setup: func(n *BaseNode, ctx context.Context) {
				n.SetSendFunc(func(to string, msg *Message) error {
					return nil
				})
				n.Start(ctx)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			node := NewBaseNode("test-node")
			ctx := context.Background()
			tt.setup(node, ctx)

			err := node.Send("target", "payload")
			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errorType)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBaseNode_Send_WithSendFunc(t *testing.T) {
	t.Parallel()
	node := NewBaseNode("test-node")
	ctx := context.Background()

	var capturedTo string
	var capturedMsg *Message

	node.SetSendFunc(func(to string, msg *Message) error {
		capturedTo = to
		capturedMsg = msg
		return nil
	})

	node.Start(ctx)

	tests := []struct {
		name    string
		to      string
		payload interface{}
	}{
		{"string payload", "target-node", "test payload"},
		{"int payload", "target-node", 42},
		{"struct payload", "target-node", struct{ X int }{X: 10}},
		{"nil payload", "target-node", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := node.Send(tt.to, tt.payload)
			require.NoError(t, err)
			assert.Equal(t, tt.to, capturedTo)
			require.NotNil(t, capturedMsg)
			assert.Equal(t, "test-node", capturedMsg.From)
			assert.Equal(t, tt.to, capturedMsg.To)
			assert.Equal(t, tt.payload, capturedMsg.Payload)
		})
	}
}

func TestBaseNode_SendMessage(t *testing.T) {
	t.Parallel()
	node := NewBaseNode("test-node")
	ctx := context.Background()

	var capturedTo string
	var capturedMsg *Message

	node.SetSendFunc(func(to string, msg *Message) error {
		capturedTo = to
		capturedMsg = msg
		return nil
	})

	node.Start(ctx)

	tests := []struct {
		name string
		msg  *Message
	}{
		{
			name: "simple message",
			msg:  NewMessage("test-node", "target-node", "custom payload"),
		},
		{
			name: "message with metadata",
			msg: func() *Message {
				msg := NewMessage("test-node", "target-node", "payload")
				msg.Metadata["key"] = "value"
				return msg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := node.SendMessage(tt.msg)
			require.NoError(t, err)
			assert.Equal(t, "target-node", capturedTo)
			assert.Equal(t, tt.msg, capturedMsg)
		})
	}
}

func TestBaseNode_EnqueueMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func(*testNode, context.Context)
		expectError bool
		errorType   error
	}{
		{
			name: "not running - error",
			setup: func(n *testNode, ctx context.Context) {
				// Don't start
			},
			expectError: true,
			errorType:   ErrNodeNotRunning,
		},
		{
			name: "running - success",
			setup: func(n *testNode, ctx context.Context) {
				n.Start(ctx)
				n.SetNodeRef(n)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testNode := newTestNode("test-node")
			ctx := context.Background()
			tt.setup(testNode, ctx)

			msg := NewMessage("sender", "test-node", "test")
			err := testNode.EnqueueMessage(msg)

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errorType)
			} else {
				require.NoError(t, err)

				select {
				case <-testNode.messageReceived:
				}

				messages := testNode.GetReceivedMessages()
				require.Len(t, messages, 1)
				assert.Equal(t, msg, messages[0])
			}
		})
	}
}

func TestBaseNode_Context(t *testing.T) {
	t.Parallel()
	node := NewBaseNode("test-node")
	ctx := context.Background()

	node.Start(ctx)

	nodeCtx := node.Context()
	require.NotNil(t, nodeCtx)

	node.Stop()

	select {
	case <-nodeCtx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after Stop()")
	}
}

func TestBaseNode_Receive_Override(t *testing.T) {
	t.Parallel()
	testNode := newTestNode("test-node")
	ctx := context.Background()

	testNode.SetNodeRef(testNode)
	testNode.Start(ctx)

	msg1 := NewMessage("sender1", "test-node", "msg1")
	msg2 := NewMessage("sender2", "test-node", "msg2")

	testNode.EnqueueMessage(msg1)
	testNode.EnqueueMessage(msg2)

	// Wait for messages using channel
	received := 0
	for received < 2 {
		<-testNode.messageReceived
		received++
	}

	messages := testNode.GetReceivedMessages()
	require.Len(t, messages, 2)
	assert.Equal(t, "msg1", messages[0].Payload)
	assert.Equal(t, "msg2", messages[1].Payload)
}
