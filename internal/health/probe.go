package health

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
)

// Prober 主动健康探测器
type Prober struct {
	timeout time.Duration
}

// NewProber 创建探测器
func NewProber(timeout time.Duration) *Prober {
	return &Prober{
		timeout: timeout,
	}
}

// Probe 探测节点健康状态
// 返回延迟和错误（如果不健康）
func (p *Prober) Probe(ctx context.Context, node *config.NodeConfig) (time.Duration, error) {
	// 创建带超时的上下文
	probeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// 记录开始时间
	startTime := time.Now()

	// 尝试建立TCP连接到trojan服务器
	addr := fmt.Sprintf("%s:%d", node.Server, node.Port)

	dialer := &net.Dialer{
		Timeout: p.timeout,
	}

	conn, err := dialer.DialContext(probeCtx, "tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	// 计算延迟
	latency := time.Since(startTime)

	// 成功建立连接即认为节点健康
	// 注意: 这里只做TCP连接检查，不做完整的Trojan握手
	// 完整的握手检查会在实际请求时进行
	return latency, nil
}

// ProbeWithRetry 带重试的探测
func (p *Prober) ProbeWithRetry(ctx context.Context, node *config.NodeConfig, retries int) (time.Duration, error) {
	var lastErr error
	var totalLatency time.Duration

	for i := 0; i < retries; i++ {
		latency, err := p.Probe(ctx, node)
		if err == nil {
			return latency, nil
		}

		lastErr = err
		totalLatency += latency

		// 如果不是最后一次重试，等待一小段时间
		if i < retries-1 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				// 继续重试
			}
		}
	}

	return totalLatency / time.Duration(retries), lastErr
}
