package health

import (
	"fmt"
	"time"
)

// Query 健康状态查询接口
type Query struct {
	checker *Checker
}

// NewQuery 创建查询接口
func NewQuery(checker *Checker) *Query {
	return &Query{
		checker: checker,
	}
}

// GetNodeStatus 查询单个节点状态
func (q *Query) GetNodeStatus(nodeName string) (*NodeStatusInfo, error) {
	status := q.checker.GetStatus(nodeName)
	if status == nil {
		return nil, fmt.Errorf("node not found: %s", nodeName)
	}

	stats := status.GetStats()
	return &NodeStatusInfo{
		NodeName:       stats.NodeName,
		Healthy:        stats.Healthy,
		LastCheck:      stats.LastCheck,
		LastSuccess:    stats.LastSuccess,
		Latency:        stats.Latency,
		LatencyMs:      stats.Latency.Milliseconds(),
		FailCount:      stats.FailCount,
		TotalRequests:  stats.TotalRequests,
		FailedRequests: stats.FailedRequests,
		SuccessRate:    stats.SuccessRate,
		ActiveConns:    stats.ActiveConns,
	}, nil
}

// GetAllNodeStatus 查询所有节点状态
func (q *Query) GetAllNodeStatus() []*NodeStatusInfo {
	statuses := q.checker.GetAllStatuses()
	result := make([]*NodeStatusInfo, 0, len(statuses))

	for _, status := range statuses {
		if status == nil {
			continue
		}

		stats := status.GetStats()
		result = append(result, &NodeStatusInfo{
			NodeName:       stats.NodeName,
			Healthy:        stats.Healthy,
			LastCheck:      stats.LastCheck,
			LastSuccess:    stats.LastSuccess,
			Latency:        stats.Latency,
			LatencyMs:      stats.Latency.Milliseconds(),
			FailCount:      stats.FailCount,
			TotalRequests:  stats.TotalRequests,
			FailedRequests: stats.FailedRequests,
			SuccessRate:    stats.SuccessRate,
			ActiveConns:    stats.ActiveConns,
		})
	}

	return result
}

// GetHealthyNodeNames 获取健康节点名称列表
func (q *Query) GetHealthyNodeNames() []string {
	nodes := q.checker.GetHealthyNodes()
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

// GetUnhealthyNodeNames 获取不健康节点名称列表
func (q *Query) GetUnhealthyNodeNames() []string {
	allStatuses := q.checker.GetAllStatuses()
	names := make([]string, 0)

	for name, status := range allStatuses {
		if !status.IsHealthy() {
			names = append(names, name)
		}
	}

	return names
}

// GetSummary 获取健康状态摘要
func (q *Query) GetSummary() HealthSummary {
	allStatuses := q.checker.GetAllStatuses()

	summary := HealthSummary{
		TotalNodes:     len(allStatuses),
		HealthyNodes:   0,
		UnhealthyNodes: 0,
		Nodes:          make([]NodeSummary, 0, len(allStatuses)),
	}

	for name, status := range allStatuses {
		stats := status.GetStats()

		nodeSummary := NodeSummary{
			Name:        name,
			Healthy:     stats.Healthy,
			Latency:     stats.Latency,
			SuccessRate: stats.SuccessRate,
		}

		summary.Nodes = append(summary.Nodes, nodeSummary)

		if stats.Healthy {
			summary.HealthyNodes++
		} else {
			summary.UnhealthyNodes++
		}
	}

	return summary
}

// NodeStatusInfo 节点状态信息（用于查询）
type NodeStatusInfo struct {
	NodeName       string        `json:"node_name"`
	Healthy        bool          `json:"healthy"`
	LastCheck      time.Time     `json:"last_check"`
	LastSuccess    time.Time     `json:"last_success"`
	Latency        time.Duration `json:"-"`
	LatencyMs      int64         `json:"latency_ms"`
	FailCount      int           `json:"fail_count"`
	TotalRequests  int64         `json:"total_requests"`
	FailedRequests int64         `json:"failed_requests"`
	SuccessRate    float64       `json:"success_rate"`
	ActiveConns    int           `json:"active_conns"`
}

// HealthSummary 健康状态摘要
type HealthSummary struct {
	TotalNodes     int           `json:"total_nodes"`
	HealthyNodes   int           `json:"healthy_nodes"`
	UnhealthyNodes int           `json:"unhealthy_nodes"`
	Nodes          []NodeSummary `json:"nodes"`
}

// NodeSummary 节点摘要
type NodeSummary struct {
	Name        string        `json:"name"`
	Healthy     bool          `json:"healthy"`
	Latency     time.Duration `json:"-"`
	SuccessRate float64       `json:"success_rate"`
}
