package health

import (
	"context"
	"sync"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Checker 健康检查器
type Checker struct {
	nodes         []*config.NodeConfig
	statuses      map[string]*NodeStatus
	interval      time.Duration
	probeTimeout  time.Duration
	failThreshold int
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	prober        *Prober
}

// NewChecker 创建健康检查器
func NewChecker(nodes []*config.NodeConfig, healthConfig config.HealthCheckConfig) *Checker {
	ctx, cancel := context.WithCancel(context.Background())

	checker := &Checker{
		nodes:         nodes,
		statuses:      make(map[string]*NodeStatus),
		interval:      healthConfig.Interval,
		probeTimeout:  healthConfig.Timeout,
		failThreshold: healthConfig.FailureThreshold,
		ctx:           ctx,
		cancel:        cancel,
		prober:        NewProber(healthConfig.Timeout),
	}

	// 为每个节点初始化状态
	for _, node := range nodes {
		checker.statuses[node.Name] = NewNodeStatus(node.Name)
	}

	return checker
}

// Start 启动健康检查器
func (c *Checker) Start() {
	logger.Info("health checker started",
		"interval", c.interval,
		"timeout", c.probeTimeout,
		"failThreshold", c.failThreshold)

	c.wg.Add(1)
	go c.checkLoop()
}

// Stop 停止健康检查器
func (c *Checker) Stop() {
	logger.Info("stopping health checker...")
	c.cancel()
	c.wg.Wait()
	logger.Info("health checker stopped")
}

// checkLoop 定期检查循环
func (c *Checker) checkLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// 启动时立即执行一次检查
	c.checkAllNodes()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkAllNodes()
		}
	}
}

// checkAllNodes 检查所有节点
func (c *Checker) checkAllNodes() {
	c.mu.RLock()
	nodes := make([]*config.NodeConfig, len(c.nodes))
	copy(nodes, c.nodes)
	c.mu.RUnlock()

	for _, node := range nodes {
		if !node.Enabled {
			continue
		}

		go c.checkNode(node)
	}
}

// checkNode 检查单个节点
func (c *Checker) checkNode(node *config.NodeConfig) {
	status := c.GetStatus(node.Name)
	if status == nil {
		return
	}

	// 执行主动探测
	latency, err := c.prober.Probe(c.ctx, node)

	if err != nil {
		status.MarkUnhealthy()
		logger.Warn("node health check failed",
			"node", node.Name,
			"error", err,
			"failCount", status.FailCount)
	} else {
		wasUnhealthy := !status.IsHealthy()
		status.MarkHealthy(latency)

		if wasUnhealthy {
			logger.Info("node recovered",
				"node", node.Name,
				"latency", latency)
		} else {
			logger.Debug("node health check success",
				"node", node.Name,
				"latency", latency)
		}
	}

	// 记录状态
	stats := status.GetStats()
	if !stats.Healthy {
		logger.Warn("node unhealthy",
			"node", node.Name,
			"failCount", stats.FailCount,
			"successRate", stats.SuccessRate,
			"lastSuccess", stats.LastSuccess)
	}
}

// GetStatus 获取节点状态
func (c *Checker) GetStatus(nodeName string) *NodeStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.statuses[nodeName]
}

// GetAllStatuses 获取所有节点状态
func (c *Checker) GetAllStatuses() map[string]*NodeStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*NodeStatus, len(c.statuses))
	for name, status := range c.statuses {
		result[name] = status
	}
	return result
}

// GetHealthyNodes 获取所有健康的节点
func (c *Checker) GetHealthyNodes() []*config.NodeConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var healthy []*config.NodeConfig
	for _, node := range c.nodes {
		if !node.Enabled {
			continue
		}

		status := c.statuses[node.Name]
		if status != nil && status.IsHealthy() {
			healthy = append(healthy, node)
		}
	}

	return healthy
}

// UpdateNodes 更新节点列表（用于配置重载）
func (c *Checker) UpdateNodes(nodes []*config.NodeConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 记录旧节点
	oldNodes := make(map[string]bool)
	for _, node := range c.nodes {
		oldNodes[node.Name] = true
	}

	// 更新节点列表
	c.nodes = nodes

	// 为新节点创建状态
	for _, node := range nodes {
		if _, exists := c.statuses[node.Name]; !exists {
			c.statuses[node.Name] = NewNodeStatus(node.Name)
			logger.Info("added new node to health checker", "node", node.Name)
		}
		delete(oldNodes, node.Name)
	}

	// 移除不再存在的节点状态
	for nodeName := range oldNodes {
		delete(c.statuses, nodeName)
		logger.Info("removed node from health checker", "node", nodeName)
	}
}

// RecordRequest 记录请求结果（被动健康检测）
func (c *Checker) RecordRequest(nodeName string, success bool, latency time.Duration) {
	status := c.GetStatus(nodeName)
	if status != nil {
		status.RecordRequest(success, latency)

		// 如果失败导致节点不健康，记录日志
		if !success && !status.IsHealthy() {
			stats := status.GetStats()
			logger.Warn("node marked unhealthy by passive check",
				"node", nodeName,
				"failCount", stats.FailCount,
				"successRate", stats.SuccessRate)
		}
	}
}
