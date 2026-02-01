package trojan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Pool manages a pool of trojan clients for a single node
type Pool struct {
	node        *config.NodeConfig
	clients     []*Client
	maxIdle     int
	maxActive   int
	timeout     time.Duration
	mu          sync.Mutex
	activeCount int
	closed      bool
}

// NewPool creates a new connection pool for a node
func NewPool(node *config.NodeConfig, maxIdle, maxActive int, timeout time.Duration) *Pool {
	return &Pool{
		node:      node,
		clients:   make([]*Client, 0, maxIdle),
		maxIdle:   maxIdle,
		maxActive: maxActive,
		timeout:   timeout,
	}
}

// Get retrieves a client from the pool or creates a new one
func (p *Pool) Get(ctx context.Context) (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, fmt.Errorf("pool is closed")
	}

	// Try to reuse an idle client
	if len(p.clients) > 0 {
		client := p.clients[len(p.clients)-1]
		p.clients = p.clients[:len(p.clients)-1]
		p.activeCount++

		// Check if client is still valid
		if !client.IsClosed() && time.Since(client.LastUsed()) < 5*time.Minute {
			logger.Debug("Reusing idle client for node %s", p.node.Name)
			return client, nil
		}

		// Client is stale, close it
		client.Close()
	}

	// Check if we can create a new client
	if p.activeCount >= p.maxActive {
		return nil, fmt.Errorf("pool is full (active: %d, max: %d)", p.activeCount, p.maxActive)
	}

	// Create new client
	logger.Debug("Creating new client for node %s", p.node.Name)
	client, err := NewClient(p.node)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Connect to trojan server
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect client: %w", err)
	}

	p.activeCount++
	return client, nil
}

// Put returns a client to the pool
func (p *Pool) Put(client *Client) {
	if client == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.activeCount--

	if p.closed || client.IsClosed() {
		client.Close()
		return
	}

	// Check if we can keep it in the idle pool
	if len(p.clients) < p.maxIdle {
		p.clients = append(p.clients, client)
		logger.Debug("Returned client to pool for node %s (idle: %d)", p.node.Name, len(p.clients))
	} else {
		// Pool is full, close the client
		client.Close()
		logger.Debug("Idle pool full for node %s, closing client", p.node.Name)
	}
}

// Close closes all clients in the pool
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Close all idle clients
	for _, client := range p.clients {
		client.Close()
	}
	p.clients = nil

	logger.Info("Closed connection pool for node %s", p.node.Name)
	return nil
}

// GetStats returns pool statistics
func (p *Pool) GetStats() (idle, active int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients), p.activeCount
}

// GetNodeName returns the name of the node this pool serves
func (p *Pool) GetNodeName() string {
	return p.node.Name
}
