package hive

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// NetworkDeliveryLoopInterval defines how often the network delivery loop checks for pending messages.
	NetworkDeliveryLoopInterval = 1 * time.Millisecond

	// FailureSimulationLoopInterval defines how often the failure simulation loop checks for crashes and recoveries.
	FailureSimulationLoopInterval = 100 * time.Millisecond
)

// Simulator coordinates the distributed system simulation.
// It manages nodes, the network, and handles time/failure models.
type Simulator struct {
	config  *Config
	network *Network
	nodes   map[string]Node
	mu      sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	running   bool
	startTime time.Time
}

// NewSimulator creates a new simulator with the given configuration.
func NewSimulator(config *Config) *Simulator {
	if config == nil {
		config = NewConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	sim := &Simulator{
		config:    config,
		nodes:     make(map[string]Node),
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	// Initialize network
	sim.network = NewNetwork(config.Network, config.Time, config.Failure)

	return sim
}

// AddNode adds a node to the simulator.
func (s *Simulator) AddNode(node Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[node.ID()]; exists {
		return fmt.Errorf("%w: %s", ErrNodeAlreadyExists, node.ID())
	}

	s.nodes[node.ID()] = node
	s.network.RegisterNode(node)

	// Set up send function and node reference for nodes that support it
	// This works for BaseNode and any node embedding BaseNode via interface
	if configurableNode, ok := node.(SendConfigurableNode); ok {
		configurableNode.SetSendFunc(func(to string, msg *Message) error {
			return s.network.Send(node.ID(), to, msg)
		})
		// Set nodeRef so processMessages can call overridden Receive methods
		configurableNode.SetNodeRef(node)
	}

	return nil
}

// RemoveNode removes a node from the simulator.
func (s *Simulator) RemoveNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}

	node.Stop()
	delete(s.nodes, nodeID)
	s.network.UnregisterNode(nodeID)

	return nil
}

// GetNode returns a node by ID.
func (s *Simulator) GetNode(nodeID string) (Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}

	return node, nil
}

// Start starts the simulator and all nodes.
func (s *Simulator) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil // Already running
	}

	s.running = true
	s.startTime = time.Now()

	// Update network startTime to match simulator startTime
	s.network.SetStartTime(s.startTime)

	// Start all nodes
	for _, node := range s.nodes {
		if err := node.Start(s.ctx); err != nil {
			return fmt.Errorf("failed to start node %s: %w", node.ID(), err)
		}
	}

	// Start network delivery loop
	s.wg.Add(1)
	go s.networkDeliveryLoop()

	// Start failure simulation loop (if failures are configured)
	// Run the loop if we have failures configured, even with 0 crash probability,
	// because we need to check for recoveries
	if s.config.Failure != nil && s.config.Failure.RecoveryAfter > 0 {
		s.wg.Add(1)
		go s.failureSimulationLoop()
	} else if s.config.Failure != nil && s.config.Failure.CrashProbability > 0.0 {
		s.wg.Add(1)
		go s.failureSimulationLoop()
	}

	return nil
}

// Stop stops the simulator and all nodes immediately.
// Pending messages in the network are not processed - the simulator stops right away.
func (s *Simulator) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	s.cancel()

	// Stop all nodes
	for _, node := range s.nodes {
		node.Stop()
	}

	s.wg.Wait()

	return nil
}

// networkDeliveryLoop continuously delivers pending messages.
func (s *Simulator) networkDeliveryLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(NetworkDeliveryLoopInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.network.ProcessMessages()
		}
	}
}

// failureSimulationLoop simulates node crashes and recoveries.
func (s *Simulator) failureSimulationLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(FailureSimulationLoopInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			nodes := make([]Node, 0, len(s.nodes))
			for _, node := range s.nodes {
				nodes = append(nodes, node)
			}
			s.mu.RUnlock()

			// Check for crashes
			for _, node := range nodes {
				if s.config.Failure.ShouldCrash(node.ID()) {
					s.config.Failure.CrashNode(node.ID())
					node.Stop()
				}
			}

			// Check for recoveries (if RecoveryAfter > 0, nodes can recover)
			if s.config.Failure.RecoveryAfter > 0 {
				crashedNodes := s.config.Failure.GetCrashedNodes()
				for nodeID := range crashedNodes {
					if s.config.Failure.ShouldRecover(nodeID) {
						s.config.Failure.RecoverNode(nodeID)

						// Restart the node
						s.mu.RLock()
						node, exists := s.nodes[nodeID]
						s.mu.RUnlock()

						if exists {
							node.Start(s.ctx)
						}
					}
				}
			}
		}
	}
}

// IsRunning returns whether the simulator is currently running.
func (s *Simulator) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetNodeCount returns the number of nodes in the simulator.
func (s *Simulator) GetNodeCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.nodes)
}

// GetCrashedNodes returns the IDs of currently crashed nodes.
func (s *Simulator) GetCrashedNodes() map[string]bool {
	if s.config.Failure == nil {
		return make(map[string]bool)
	}
	return s.config.Failure.GetCrashedNodes()
}

// GetPendingMessageCount returns the number of messages waiting to be delivered.
func (s *Simulator) GetPendingMessageCount() int {
	return s.network.GetPendingMessageCount()
}
