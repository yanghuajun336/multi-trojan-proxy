package health

import (
	"time"

	"github.com/yanghuajun/proxy/pkg/logger"
)

// PassiveDetector 被动健康检测器
// 通过监控实际请求的成功/失败来快速发现问题
type PassiveDetector struct {
	checker *Checker
}

// NewPassiveDetector 创建被动检测器
func NewPassiveDetector(checker *Checker) *PassiveDetector {
	return &PassiveDetector{
		checker: checker,
	}
}

// RecordSuccess 记录成功的请求
func (pd *PassiveDetector) RecordSuccess(nodeName string, latency time.Duration) {
	pd.checker.RecordRequest(nodeName, true, latency)
}

// RecordFailure 记录失败的请求
func (pd *PassiveDetector) RecordFailure(nodeName string, err error) {
	pd.checker.RecordRequest(nodeName, false, 0)

	logger.Debug("passive detection: request failed",
		"node", nodeName,
		"error", err)
}

// RecordConnectionStart 记录连接开始
func (pd *PassiveDetector) RecordConnectionStart(nodeName string) {
	status := pd.checker.GetStatus(nodeName)
	if status != nil {
		status.IncrementActiveConns()
	}
}

// RecordConnectionEnd 记录连接结束
func (pd *PassiveDetector) RecordConnectionEnd(nodeName string) {
	status := pd.checker.GetStatus(nodeName)
	if status != nil {
		status.DecrementActiveConns()
	}
}
