package trojan

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
	"github.com/yanghuajun/proxy/internal/health"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	GetStatus(nodeName string) *health.NodeStatus
	GetHealthyNodes() []*config.NodeConfig
}

// Selector selects a trojan node for load balancing
type Selector struct {
	pools         map[string]*Pool
	nodes         []*config.NodeConfig
	current       int
	healthChecker HealthChecker
	poolConfig    config.PoolConfig
	mu            sync.Mutex
}

// NewSelector creates a new node selector
func NewSelector(nodes []config.NodeConfig, poolConfig config.PoolConfig) (*Selector, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes provided")
	}

	s := &Selector{
		pools:      make(map[string]*Pool),
		nodes:      make([]*config.NodeConfig, 0, len(nodes)),
		poolConfig: poolConfig,
	}

	// Create pools for enabled nodes
	for i := range nodes {
		node := &nodes[i]
		if !node.Enabled {
			logger.Info("Skipping disabled node: %s", node.Name)
			continue
		}

		// Create connection pool for this node with configured sizes
		pool := NewPool(node, poolConfig.MaxIdle, poolConfig.MaxActive, 10*time.Second)
		s.pools[node.Name] = pool
		s.nodes = append(s.nodes, node)

		logger.Info("Created connection pool for node: %s (weight=%d, maxIdle=%d, maxActive=%d)",
			node.Name, node.Weight, poolConfig.MaxIdle, poolConfig.MaxActive)
	}

	if len(s.nodes) == 0 {
		return nil, fmt.Errorf("no enabled nodes")
	}

	return s, nil
}

// SelectClient selects a client using weighted round-robin load balancing
// with health check support
func (s *Selector) SelectClient() (*Client, error) {
	s.mu.Lock()

	if len(s.nodes) == 0 {
		s.mu.Unlock()
		return nil, fmt.Errorf("no available nodes")
	}

	// Get healthy nodes if health checker is available
	var availableNodes []*config.NodeConfig
	if s.healthChecker != nil {
		availableNodes = s.healthChecker.GetHealthyNodes()
		if len(availableNodes) == 0 {
			// Fallback to all nodes if no healthy nodes
			logger.Warn("no healthy nodes available, falling back to all nodes")
			availableNodes = s.nodes
		}
	} else {
		availableNodes = s.nodes
	}

	if len(availableNodes) == 0 {
		s.mu.Unlock()
		return nil, fmt.Errorf("no available nodes")
	}

	// Weighted round-robin selection
	node := s.selectWeightedNode(availableNodes)
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
		logger.Warn("failed to get client from pool",
			"node", node.Name,
			"error", err)
		// Try next available node
		return s.selectFromOtherNodes(availableNodes, node.Name)
	}

	logger.Debug("selected node for request", "node", node.Name)
	return client, nil
}

// selectWeightedNode 使用加权轮询选择节点
func (s *Selector) selectWeightedNode(nodes []*config.NodeConfig) *config.NodeConfig {
	if len(nodes) == 1 {
		return nodes[0]
	}

	// 计算总权重
	totalWeight := 0
	for _, n := range nodes {
		totalWeight += n.Weight
	}

	if totalWeight == 0 {
		// 如果所有权重都是0，使用简单轮询
		node := nodes[s.current%len(nodes)]
		s.current++
		return node
	}

	// 加权轮询算法
	s.current++
	offset := s.current % totalWeight

	cumulative := 0
	for _, n := range nodes {
		cumulative += n.Weight
		if offset < cumulative {
			return n
		}
	}

	// 不应该到达这里，但以防万一
	return nodes[0]
}

// selectFromOtherNodes 从其他节点选择（故障转移）
func (s *Selector) selectFromOtherNodes(availableNodes []*config.NodeConfig, failedNode string) (*Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, node := range availableNodes {
		if node.Name == failedNode {
			continue
		}

		pool := s.pools[node.Name]
		if pool == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err := pool.Get(ctx)
		cancel()

		if err == nil {
			logger.Info("failover to node",
				"from", failedNode,
				"to", node.Name)
			return client, nil
		}
	}

	return nil, fmt.Errorf("no available nodes after failover")
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

// SetHealthChecker 设置健康检查器
func (s *Selector) SetHealthChecker(checker HealthChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.healthChecker = checker
	logger.Info("health checker attached to selector")
}

// UpdateNodes 更新节点配置（用于配置重载）
func (s *Selector) UpdateNodes(nodes []config.NodeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 记录旧节点
	oldPools := make(map[string]*Pool)
	for name, pool := range s.pools {
		oldPools[name] = pool
	}

	// 准备新的节点列表和连接池
	newNodes := make([]*config.NodeConfig, 0, len(nodes))
	newPools := make(map[string]*Pool)

	for i := range nodes {
		node := &nodes[i]
		if !node.Enabled {
			logger.Info("skipping disabled node", "node", node.Name)
			continue
		}

		// 如果节点已存在，复用连接池
		if existingPool, exists := oldPools[node.Name]; exists {
			newPools[node.Name] = existingPool
			delete(oldPools, node.Name)
			logger.Info("reusing pool for existing node", "node", node.Name)
		} else {
			// 创建新的连接池（使用配置的连接池大小）
			pool := NewPool(node, s.poolConfig.MaxIdle, s.poolConfig.MaxActive, 10*time.Second)
			newPools[node.Name] = pool
			logger.Info("created pool for new node",
				"node", node.Name,
				"weight", node.Weight,
				"maxIdle", s.poolConfig.MaxIdle,
				"maxActive", s.poolConfig.MaxActive)
		}

		newNodes = append(newNodes, node)
	}

	// 关闭被移除节点的连接池
	for name, pool := range oldPools {
		pool.Close()
		logger.Info("closed pool for removed node", "node", name)
	}

	// 更新状态
	s.nodes = newNodes
	s.pools = newPools

	if len(s.nodes) == 0 {
		return fmt.Errorf("no enabled nodes after update")
	}

	return nil
}
