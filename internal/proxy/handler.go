package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/yanghuajun/proxy/internal/health"
	"github.com/yanghuajun/proxy/internal/router"
	"github.com/yanghuajun/proxy/internal/trojan"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Handler handles HTTP and HTTPS proxy requests
type Handler struct {
	selector        *trojan.Selector
	timeout         time.Duration
	failover        *Failover
	passiveDetector *health.PassiveDetector
	router          *router.Router
}

// NewHandler creates a new proxy handler
func NewHandler(selector *trojan.Selector, timeout time.Duration) *Handler {
	return &Handler{
		selector: selector,
		timeout:  timeout,
	}
}

// SetFailover 设置故障转移管理器
func (h *Handler) SetFailover(failover *Failover) {
	h.failover = failover
}

// SetPassiveDetector 设置被动检测器
func (h *Handler) SetPassiveDetector(detector *health.PassiveDetector) {
	h.passiveDetector = detector
}

// SetRouter 设置路由器
func (h *Handler) SetRouter(r *router.Router) {
	h.router = r
}

// ServeHTTP handles both HTTP and HTTPS (CONNECT) requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := r.Host
	if r.Method != http.MethodConnect {
		target = r.URL.Host
		if target == "" {
			target = r.Host
		}
	}

	// 路由决策
	action := router.ActionProxy // 默认走代理
	if h.router != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		routedAction, err := h.router.Route(ctx, target)
		if err != nil {
			logger.Warn("routing decision failed, using default",
				"target", target,
				"error", err)
		} else {
			action = routedAction
		}
	}

	// 根据路由动作处理请求
	switch action {
	case router.ActionDirect:
		// 直接连接，不使用代理
		h.handleDirect(w, r)
	case router.ActionProxy:
		// 通过代理连接
		if r.Method == http.MethodConnect {
			h.handleConnect(w, r)
		} else {
			h.handleHTTP(w, r)
		}
	case router.ActionReject:
		// 拒绝连接
		logger.Info("request rejected by routing rule", "target", target)
		http.Error(w, "Request rejected by routing policy", http.StatusForbidden)
	default:
		logger.Warn("unknown routing action, falling back to proxy",
			"action", action,
			"target", target)
		if r.Method == http.MethodConnect {
			h.handleConnect(w, r)
		} else {
			h.handleHTTP(w, r)
		}
	}
}

// handleConnect handles HTTPS CONNECT requests
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	logger.Debug("CONNECT request", "target", r.Host)

	startTime := time.Now()

	// Get a trojan client from the pool
	client, err := h.selector.SelectClient()
	if err != nil {
		logger.Error("failed to select trojan client", "error", err)
		http.Error(w, "No available proxy nodes", http.StatusServiceUnavailable)
		return
	}

	nodeName := client.GetNodeName()

	// Record connection start
	if h.passiveDetector != nil {
		h.passiveDetector.RecordConnectionStart(nodeName)
		defer h.passiveDetector.RecordConnectionEnd(nodeName)
	}

	// Hijack the client connection FIRST (before slow Trojan connection)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logger.Error("ResponseWriter does not support hijacking")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		client.Close()
		h.selector.ReleaseClient(client)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logger.Error("failed to hijack connection", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		client.Close()
		h.selector.ReleaseClient(client)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection Established response IMMEDIATELY
	// This prevents client timeout while we establish Trojan connection
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// NOW connect to the target through trojan (may be slow)
	targetConn, err := client.Dial(r.Context(), "tcp", r.Host)
	if err != nil {
		logger.Error("failed to connect via trojan",
			"target", r.Host,
			"node", nodeName,
			"error", err)

		// Record failure
		if h.passiveDetector != nil {
			h.passiveDetector.RecordFailure(nodeName, err)
		}

		client.Close()
		h.selector.ReleaseClient(client)
		// clientConn will be closed by defer
		return
	}
	defer targetConn.Close()

	// Start bidirectional copy
	logger.Debug("tunneling connection",
		"target", r.Host,
		"node", nodeName)

	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(targetConn, clientConn)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(clientConn, targetConn)
		errCh <- err
	}()

	// Wait for one direction to complete
	err = <-errCh
	latency := time.Since(startTime)

	// Record success/failure
	if h.passiveDetector != nil {
		if err != nil && err != io.EOF {
			h.passiveDetector.RecordFailure(nodeName, err)
			logger.Debug("tunnel error", "target", r.Host, "error", err)
		} else {
			h.passiveDetector.RecordSuccess(nodeName, latency)
		}
	}

	h.selector.ReleaseClient(client)
	logger.Debug("CONNECT tunnel closed",
		"target", r.Host,
		"duration", latency)
}

// handleHTTP handles plain HTTP requests
func (h *Handler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Debug("HTTP request", "url", r.URL.String(), "host", r.Host)

	startTime := time.Now()

	// Get a trojan client from the pool
	client, err := h.selector.SelectClient()
	if err != nil {
		logger.Error("failed to select trojan client", "error", err)
		http.Error(w, "No available proxy nodes", http.StatusServiceUnavailable)
		return
	}

	nodeName := client.GetNodeName()

	// Record connection start
	if h.passiveDetector != nil {
		h.passiveDetector.RecordConnectionStart(nodeName)
		defer h.passiveDetector.RecordConnectionEnd(nodeName)
	}

	// Remove hop-by-hop headers
	removeHopByHopHeaders(r.Header)

	// Create a new request to forward through trojan
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	
	// Ensure the URL has a scheme and host for HTTP proxy requests
	if outReq.URL.Scheme == "" {
		outReq.URL.Scheme = "http"
	}
	if outReq.URL.Host == "" {
		outReq.URL.Host = r.Host
	}

	// Forward the request through trojan
	err = forwardRequest(client, outReq, w)

	latency := time.Since(startTime)

	// Record success/failure
	if h.passiveDetector != nil {
		if err != nil {
			h.passiveDetector.RecordFailure(nodeName, err)
		} else {
			h.passiveDetector.RecordSuccess(nodeName, latency)
		}
	}

	h.selector.ReleaseClient(client)

	if err != nil {
		logger.Error("failed to forward HTTP request",
			"url", r.URL.String(),
			"node", nodeName,
			"error", err)
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}

	logger.Debug("HTTP request completed",
		"url", r.URL.String(),
		"duration", latency)
}

// removeHopByHopHeaders removes hop-by-hop headers
func removeHopByHopHeaders(header http.Header) {
	hopByHopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopByHopHeaders {
		header.Del(h)
	}
}

// handleDirect 处理直接连接（不使用代理）
func (h *Handler) handleDirect(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.handleDirectConnect(w, r)
	} else {
		h.handleDirectHTTP(w, r)
	}
}

// handleDirectConnect 直接CONNECT隧道（不使用trojan）
func (h *Handler) handleDirectConnect(w http.ResponseWriter, r *http.Request) {
	logger.Debug("DIRECT CONNECT request", "target", r.Host)

	// 直接连接到目标
	targetConn, err := net.DialTimeout("tcp", r.Host, h.timeout)
	if err != nil {
		logger.Error("failed to connect directly",
			"target", r.Host,
			"error", err)
		http.Error(w, "Failed to connect to target", http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	// Hijack客户端连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		logger.Error("ResponseWriter does not support hijacking")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		logger.Error("failed to hijack connection", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// 发送200响应
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// 双向转发
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(targetConn, clientConn)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(clientConn, targetConn)
		errCh <- err
	}()

	<-errCh
	logger.Debug("DIRECT CONNECT tunnel closed", "target", r.Host)
}

// handleDirectHTTP 直接HTTP请求（不使用trojan）
func (h *Handler) handleDirectHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Debug("DIRECT HTTP request", "url", r.URL.String())

	// 创建直接请求
	client := &http.Client{
		Timeout: h.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 复制请求
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	removeHopByHopHeaders(outReq.Header)

	// 发送请求
	resp, err := client.Do(outReq)
	if err != nil {
		logger.Error("direct HTTP request failed",
			"url", r.URL.String(),
			"error", err)
		http.Error(w, "Failed to forward request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 写入状态码
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	io.Copy(w, resp.Body)

	logger.Debug("DIRECT HTTP request completed", "url", r.URL.String())
}
