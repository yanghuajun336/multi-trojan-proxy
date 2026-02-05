package trojan

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
)

// Client represents a Trojan client connection
type Client struct {
	config    *config.NodeConfig
	conn      net.Conn
	createdAt time.Time
	lastUsed  time.Time
	closed    bool
}

// NewClient creates a new Trojan client
func NewClient(nodeConfig *config.NodeConfig) (*Client, error) {
	if nodeConfig == nil {
		return nil, fmt.Errorf("node config is nil")
	}

	return &Client{
		config:    nodeConfig,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		closed:    false,
	}, nil
}

// Connect establishes a connection to the Trojan server
func (c *Client) Connect(ctx context.Context) error {
	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// Build server address
	serverAddr := fmt.Sprintf("%s:%d", c.config.Server, c.config.Port)

	// Set connection timeout
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	// Establish TCP connection
	conn, err := dialer.DialContext(ctx, "tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to trojan server: %w", err)
	}

	c.conn = conn
	c.lastUsed = time.Now()

	return nil
}

// Dial establishes a connection to the target through the Trojan tunnel
func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Establish a complete Trojan connection with TLS and protocol handshake
	conn, err := dialTrojan(c.config, address)
	if err != nil {
		return nil, fmt.Errorf("trojan dial failed for %s: %w", address, err)
	}

	c.conn = conn
	c.lastUsed = time.Now()

	return conn, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.closed {
		return nil
	}

	c.closed = true
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsClosed returns whether the client is closed
func (c *Client) IsClosed() bool {
	return c.closed
}

// LastUsed returns the last time this client was used
func (c *Client) LastUsed() time.Time {
	return c.lastUsed
}

// GetNodeName returns the name of the node this client connects to
func (c *Client) GetNodeName() string {
	return c.config.Name
}
