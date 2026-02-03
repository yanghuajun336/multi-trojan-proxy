package proxy

import (
	"context"
	"fmt"
	"time"

	"github.com/yanghuajun/proxy/internal/trojan"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Failover 故障转移管理器
type Failover struct {
	selector      *trojan.Selector
	maxRetries    int
	retryDelay    time.Duration
	requestLogger RequestLogger
}

// RequestLogger 请求日志记录器接口
type RequestLogger interface {
	RecordSuccess(nodeName string, latency time.Duration)
	RecordFailure(nodeName string, err error)
}

// NewFailover 创建故障转移管理器
func NewFailover(selector *trojan.Selector, maxRetries int, requestLogger RequestLogger) *Failover {
	return &Failover{
		selector:      selector,
		maxRetries:    maxRetries,
		retryDelay:    100 * time.Millisecond,
		requestLogger: requestLogger,
	}
}

// ExecuteWithFailover 执行带故障转移的操作
func (f *Failover) ExecuteWithFailover(
	ctx context.Context,
	operation func(*trojan.Client) error,
) error {
	var lastErr error
	triedNodes := make(map[string]bool)

	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		// 选择客户端
		client, err := f.selector.SelectClient()
		if err != nil {
			return fmt.Errorf("failed to select client: %w", err)
		}

		nodeName := client.GetNodeName()

		// 如果已经尝试过这个节点，跳过
		if triedNodes[nodeName] && attempt > 0 {
			f.selector.ReleaseClient(client)
			continue
		}

		triedNodes[nodeName] = true
		startTime := time.Now()

		// 执行操作
		err = operation(client)

		latency := time.Since(startTime)

		if err == nil {
			// 成功，记录并返回
			f.selector.ReleaseClient(client)
			if f.requestLogger != nil {
				f.requestLogger.RecordSuccess(nodeName, latency)
			}

			if attempt > 0 {
				logger.Info("request succeeded after failover",
					"node", nodeName,
					"attempts", attempt+1,
					"latency", latency)
			}

			return nil
		}

		// 失败，记录错误
		lastErr = err
		client.Close() // 不归还到池中，关闭连接
		f.selector.ReleaseClient(client)

		if f.requestLogger != nil {
			f.requestLogger.RecordFailure(nodeName, err)
		}

		logger.Warn("request failed, attempting failover",
			"node", nodeName,
			"attempt", attempt+1,
			"maxRetries", f.maxRetries,
			"error", err)

		// 如果还有重试机会，等待一段时间
		if attempt < f.maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(f.retryDelay):
				// 继续重试
			}
		}
	}

	return fmt.Errorf("all failover attempts exhausted: %w", lastErr)
}

// SelectClientWithFailover 选择客户端，失败时自动尝试其他节点
func (f *Failover) SelectClientWithFailover(ctx context.Context) (*trojan.Client, error) {
	var lastErr error

	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		client, err := f.selector.SelectClient()
		if err == nil {
			return client, nil
		}

		lastErr = err
		logger.Warn("failed to select client, retrying",
			"attempt", attempt+1,
			"error", err)

		if attempt < f.maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(f.retryDelay):
				// 继续重试
			}
		}
	}

	return nil, fmt.Errorf("failed to select client after %d attempts: %w", f.maxRetries+1, lastErr)
}
