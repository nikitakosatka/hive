package hive

import "errors"

var (
	ErrNodeNotRunning    = errors.New("node is not running")
	ErrNodeStopped       = errors.New("node has been stopped")
	ErrMessageQueueFull  = errors.New("message queue is full")
	ErrNoSendFunction    = errors.New("send function not set")
	ErrNodeNotFound      = errors.New("node not found")
	ErrNodeAlreadyExists = errors.New("node with this ID already exists")
	ErrInvalidConfig     = errors.New("invalid configuration")
)
