package trojan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Selector selects a trojan node for load balancing
type Selector struct {
	pools   map[string]*Pool
	nodes   []*config.NodeConfig
	current int
	mu      sync.Mutex
}

// NewSelector creates a new node selector
func NewSelector(nodes []config.NodeConfig) (*Selector, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes provided")
	}

	s := &Selector{
		pools: make(map[string]*Pool),
		nodes: make([]*config.NodeConfig, 0, len(nodes)),
	}

	// Create pools for enabled nodes
	for i := range nodes {
		node := &nodes[i]
		if !node.Enabled {
			logger.Info("Skipping disabled node: %s", node.Name)
			continue
		}

		// Create connection pool for this node
		pool := NewPool(node, 5, 10, 10*time.Second)
		s.pools[node.Name] = pool
		s.nodes = append(s.nodes, node)

		logger.Info("Created connection pool for node: %s (weight=%d)", node.Name, node.Weight)
	}

	if len(s.nodes) == 0 {
		return nil, fmt.Errorf("no enabled nodes")
	}

	return s, nil
}

// SelectClient selects a client using round-robin load balancing
func (s *Selector) SelectClient() (*Client, error) {
	s.mu.Lock()
	
	if len(s.nodes) == 0 {
		s.mu.Unlock()
		return nil, fmt.Errorf("no available nodes")
	}

	// Simple round-robin selection
	// TODO: Implement weighted round-robin based on node weights
	node := s.nodes[s.current]
	s.current = (s.current + 1) % len(s.nodes)
	
	pool := s.pools[node.Name]
	s.mu.Unlock()

	if pool == nil {
		return nil, fmt.Errorf("no pool for node %s", node.Name)
	}

	// Get client from pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := pool.Get(ctx)
	if err != nil {
		logger.Warn("Failed to get client from pool for node %s: %v", node.Name, err)
		// TODO: Try next node in case of failure
		return nil, err
	}

	logger.Debug("Selected node %s for request", node.Name)
	return client, nil
}

// ReleaseClient returns a client to its pool
func (s *Selector) ReleaseClient(client *Client) {
	if client == nil {
		return
	}

	nodeName := client.GetNodeName()
	
	s.mu.Lock()
	pool := s.pools[nodeName]
	s.mu.Unlock()

	if pool != nil {
		pool.Put(client)
	} else {
		client.Close()
	}
}

// Close closes all connection pools
func (s *Selector) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pool := range s.pools {
		pool.Close()
	}

	s.pools = nil
	s.nodes = nil

	logger.Info("Closed all connection pools")
	return nil
}

// GetPoolStats returns statistics for all pools
func (s *Selector) GetPoolStats() map[string]struct{ Idle, Active int } {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := make(map[string]struct{ Idle, Active int })
	for name, pool := range s.pools {
		idle, active := pool.GetStats()
		stats[name] = struct{ Idle, Active int }{Idle: idle, Active: active}
	}

	return stats
}
