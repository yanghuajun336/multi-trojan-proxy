package health

import (
	"sync"
	"time"
)

// NodeStatus 节点健康状态
type NodeStatus struct {
	NodeName       string        // 关联的节点名称
	Healthy        bool          // 当前健康状态
	LastCheck      time.Time     // 最后一次健康检查时间
	LastSuccess    time.Time     // 最后一次成功时间
	Latency        time.Duration // 平均延迟
	FailCount      int           // 连续失败次数
	TotalRequests  int64         // 总请求数
	FailedRequests int64         // 失败请求数
	SuccessRate    float64       // 成功率 (0-1)
	ActiveConns    int           // 当前活跃连接数
	mu             sync.RWMutex  // 保护并发访问
}

// NewNodeStatus 创建新的节点状态
func NewNodeStatus(nodeName string) *NodeStatus {
	return &NodeStatus{
		NodeName:    nodeName,
		Healthy:     true, // 初始状态为健康
		LastCheck:   time.Now(),
		LastSuccess: time.Now(),
		Latency:     0,
		FailCount:   0,
		SuccessRate: 1.0,
	}
}

// MarkHealthy 标记节点为健康
func (ns *NodeStatus) MarkHealthy(latency time.Duration) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.Healthy = true
	ns.FailCount = 0
	ns.LastCheck = time.Now()
	ns.LastSuccess = time.Now()

	// 使用指数移动平均计算延迟
	if ns.Latency == 0 {
		ns.Latency = latency
	} else {
		// EMA: new = alpha * current + (1-alpha) * old, alpha = 0.3
		ns.Latency = time.Duration(0.3*float64(latency) + 0.7*float64(ns.Latency))
	}
}

// MarkUnhealthy 标记节点为不健康
func (ns *NodeStatus) MarkUnhealthy() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.FailCount++
	ns.LastCheck = time.Now()

	// 连续失败3次标记为不健康
	if ns.FailCount >= 3 {
		ns.Healthy = false
	}
}

// RecordRequest 记录请求结果（被动健康检测）
func (ns *NodeStatus) RecordRequest(success bool, latency time.Duration) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.TotalRequests++
	if !success {
		ns.FailedRequests++
		ns.FailCount++

		// 快速标记不健康
		if ns.FailCount >= 3 {
			ns.Healthy = false
		}
	} else {
		ns.FailCount = 0
		ns.LastSuccess = time.Now()

		// 更新延迟
		if ns.Latency == 0 {
			ns.Latency = latency
		} else {
			ns.Latency = time.Duration(0.3*float64(latency) + 0.7*float64(ns.Latency))
		}
	}

	// 计算成功率
	if ns.TotalRequests > 0 {
		ns.SuccessRate = float64(ns.TotalRequests-ns.FailedRequests) / float64(ns.TotalRequests)
	}
}

// IncrementActiveConns 增加活跃连接数
func (ns *NodeStatus) IncrementActiveConns() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.ActiveConns++
}

// DecrementActiveConns 减少活跃连接数
func (ns *NodeStatus) DecrementActiveConns() {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.ActiveConns > 0 {
		ns.ActiveConns--
	}
}

// IsHealthy 获取健康状态
func (ns *NodeStatus) IsHealthy() bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.Healthy
}

// GetLatency 获取延迟
func (ns *NodeStatus) GetLatency() time.Duration {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.Latency
}

// GetSuccessRate 获取成功率
func (ns *NodeStatus) GetSuccessRate() float64 {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.SuccessRate
}

// GetActiveConns 获取活跃连接数
func (ns *NodeStatus) GetActiveConns() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.ActiveConns
}

// GetStats 获取完整统计信息（用于日志和监控）
func (ns *NodeStatus) GetStats() StatusStats {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return StatusStats{
		NodeName:       ns.NodeName,
		Healthy:        ns.Healthy,
		LastCheck:      ns.LastCheck,
		LastSuccess:    ns.LastSuccess,
		Latency:        ns.Latency,
		FailCount:      ns.FailCount,
		TotalRequests:  ns.TotalRequests,
		FailedRequests: ns.FailedRequests,
		SuccessRate:    ns.SuccessRate,
		ActiveConns:    ns.ActiveConns,
	}
}

// StatusStats 状态统计信息（只读副本）
type StatusStats struct {
	NodeName       string
	Healthy        bool
	LastCheck      time.Time
	LastSuccess    time.Time
	Latency        time.Duration
	FailCount      int
	TotalRequests  int64
	FailedRequests int64
	SuccessRate    float64
	ActiveConns    int
}
