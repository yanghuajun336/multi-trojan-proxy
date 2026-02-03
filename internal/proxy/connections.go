package proxy

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/yanghuajun/proxy/pkg/logger"
)

// ConnectionTracker 客户端连接跟踪器
type ConnectionTracker struct {
	connections map[string]*ConnectionInfo
	totalConns  int64
	mu          sync.RWMutex
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	ID           string
	RemoteAddr   string
	ConnectedAt  time.Time
	LastActivity time.Time
	BytesSent    int64
	BytesRecv    int64
	Target       string
	NodeName     string
}

// NewConnectionTracker 创建连接跟踪器
func NewConnectionTracker() *ConnectionTracker {
	tracker := &ConnectionTracker{
		connections: make(map[string]*ConnectionInfo),
	}

	// 启动清理goroutine
	go tracker.cleanupLoop()

	return tracker
}

// TrackConnection 跟踪新连接
func (ct *ConnectionTracker) TrackConnection(id, remoteAddr, target string) *ConnectionInfo {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	info := &ConnectionInfo{
		ID:           id,
		RemoteAddr:   remoteAddr,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
		Target:       target,
	}

	ct.connections[id] = info
	atomic.AddInt64(&ct.totalConns, 1)

	logger.Debug("tracking new connection",
		"id", id,
		"remote", remoteAddr,
		"target", target)

	return info
}

// UntrackConnection 移除连接跟踪
func (ct *ConnectionTracker) UntrackConnection(id string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if info, exists := ct.connections[id]; exists {
		duration := time.Since(info.ConnectedAt)
		logger.Debug("connection closed",
			"id", id,
			"duration", duration,
			"sent", info.BytesSent,
			"recv", info.BytesRecv)

		delete(ct.connections, id)
	}
}

// UpdateActivity 更新连接活动时间
func (ct *ConnectionTracker) UpdateActivity(id string) {
	ct.mu.RLock()
	info := ct.connections[id]
	ct.mu.RUnlock()

	if info != nil {
		info.LastActivity = time.Now()
	}
}

// UpdateTraffic 更新流量统计
func (ct *ConnectionTracker) UpdateTraffic(id string, sent, recv int64) {
	ct.mu.RLock()
	info := ct.connections[id]
	ct.mu.RUnlock()

	if info != nil {
		atomic.AddInt64(&info.BytesSent, sent)
		atomic.AddInt64(&info.BytesRecv, recv)
	}
}

// SetNode 设置连接使用的节点
func (ct *ConnectionTracker) SetNode(id, nodeName string) {
	ct.mu.RLock()
	info := ct.connections[id]
	ct.mu.RUnlock()

	if info != nil {
		info.NodeName = nodeName
	}
}

// GetActiveConnections 获取活跃连接数
func (ct *ConnectionTracker) GetActiveConnections() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.connections)
}

// GetTotalConnections 获取总连接数
func (ct *ConnectionTracker) GetTotalConnections() int64 {
	return atomic.LoadInt64(&ct.totalConns)
}

// GetConnectionInfo 获取连接信息
func (ct *ConnectionTracker) GetConnectionInfo(id string) *ConnectionInfo {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.connections[id]
}

// GetAllConnections 获取所有活跃连接
func (ct *ConnectionTracker) GetAllConnections() []*ConnectionInfo {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	conns := make([]*ConnectionInfo, 0, len(ct.connections))
	for _, info := range ct.connections {
		conns = append(conns, info)
	}

	return conns
}

// GetStats 获取统计信息
func (ct *ConnectionTracker) GetStats() ConnectionStats {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	stats := ConnectionStats{
		ActiveConnections: len(ct.connections),
		TotalConnections:  atomic.LoadInt64(&ct.totalConns),
		ByNode:            make(map[string]int),
	}

	var totalSent, totalRecv int64
	for _, info := range ct.connections {
		totalSent += atomic.LoadInt64(&info.BytesSent)
		totalRecv += atomic.LoadInt64(&info.BytesRecv)

		if info.NodeName != "" {
			stats.ByNode[info.NodeName]++
		}
	}

	stats.TotalBytesSent = totalSent
	stats.TotalBytesRecv = totalRecv

	return stats
}

// cleanupLoop 定期清理空闲连接记录
func (ct *ConnectionTracker) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ct.mu.Lock()
		now := time.Now()
		staleConnections := make([]string, 0)

		for id, info := range ct.connections {
			// 如果连接超过1小时没有活动，认为是僵尸连接并清理
			if now.Sub(info.LastActivity) > 1*time.Hour {
				staleConnections = append(staleConnections, id)
			}
		}

		for _, id := range staleConnections {
			logger.Warn("cleaning up stale connection", "id", id)
			delete(ct.connections, id)
		}
		ct.mu.Unlock()

		if len(staleConnections) > 0 {
			logger.Info("cleaned up stale connections", "count", len(staleConnections))
		}
	}
}

// ConnectionStats 连接统计信息
type ConnectionStats struct {
	ActiveConnections int
	TotalConnections  int64
	TotalBytesSent    int64
	TotalBytesRecv    int64
	ByNode            map[string]int
}
