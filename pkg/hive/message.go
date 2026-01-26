package hive

import (
	"time"
)

// Message represents a message sent between nodes in the distributed system.
// Students can extend this or wrap it to implement logical clocks (Lamport, vector clocks, etc.)
type Message struct {
	// From is the ID of the sending node
	From string

	// To is the ID of the receiving node
	To string

	// Payload is the actual message data (can be any type)
	Payload interface{}

	// Timestamp is the real-time when the message was created
	Timestamp time.Time

	// Metadata allows students to add custom fields (e.g., logical clock values)
	Metadata map[string]interface{}
}

// NewMessage creates a new message with the given sender, receiver, and payload.
func NewMessage(from, to string, payload interface{}) *Message {
	return &Message{
		From:      from,
		To:        to,
		Payload:   payload,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}
