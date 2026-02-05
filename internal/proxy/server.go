package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Server represents the HTTP proxy server
type Server struct {
	config     *config.ProxyConfig
	server     *http.Server
	handler    *Handler
	activeConns int
	maxConns   int
	mu         sync.Mutex
	stopCh     chan struct{}
	connTracker *ConnectionTracker
}

// NewServer creates a new proxy server
func NewServer(cfg *config.Config, handler *Handler) *Server {
	s := &Server{
		config:      &cfg.Proxy,
		handler:     handler,
		maxConns:    cfg.Proxy.MaxConcurrent,
		stopCh:      make(chan struct{}),
		connTracker: NewConnectionTracker(),
	}

	// Create HTTP server
	s.server = &http.Server{
		Addr:         cfg.Proxy.Listen,
		Handler:      s,
		ReadTimeout:  cfg.Proxy.Timeout,
		WriteTimeout: cfg.Proxy.Timeout,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// ServeHTTP implements http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check concurrent connection limit
	if !s.acquireConn() {
		logger.Warn("Max concurrent connections reached, rejecting request from %s", r.RemoteAddr)
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	defer s.releaseConn()

	// Log request
	logger.Debug("Request from %s: %s %s", r.RemoteAddr, r.Method, r.URL.String())

	// Delegate to handler
	s.handler.ServeHTTP(w, r)
}

// acquireConn tries to acquire a connection slot
func (s *Server) acquireConn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeConns >= s.maxConns {
		return false
	}

	s.activeConns++
	return true
}

// releaseConn releases a connection slot
func (s *Server) releaseConn() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeConns > 0 {
		s.activeConns--
	}
}

// Start starts the proxy server
func (s *Server) Start() error {
	logger.Info("Starting HTTP proxy server on %s", s.config.Listen)
	logger.Info("Max concurrent connections: %d", s.maxConns)
	logger.Info("Request timeout: %v", s.config.Timeout)

	// Start listening
	listener, err := net.Listen("tcp", s.config.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Listen, err)
	}

	logger.Info("Proxy server started successfully")

	// Start serving in a goroutine
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("Proxy server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the proxy server gracefully
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Stopping proxy server...")

	// Shutdown the HTTP server
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	close(s.stopCh)
	logger.Info("Proxy server stopped")
	return nil
}

// GetActiveConnections returns the number of active connections
func (s *Server) GetActiveConnections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeConns
}

// GetConnectionTracker 获取连接跟踪器
func (s *Server) GetConnectionTracker() *ConnectionTracker {
	return s.connTracker
}

// GetStats 获取服务器统计信息
func (s *Server) GetStats() ServerStats {
	s.mu.Lock()
	activeConns := s.activeConns
	s.mu.Unlock()

	connStats := s.connTracker.GetStats()

	return ServerStats{
		ActiveConnections: activeConns,
		TotalConnections:  connStats.TotalConnections,
		MaxConnections:    s.maxConns,
		TotalBytesSent:    connStats.TotalBytesSent,
		TotalBytesRecv:    connStats.TotalBytesRecv,
		ConnectionsByNode: connStats.ByNode,
	}
}

// ServerStats 服务器统计信息
type ServerStats struct {
	ActiveConnections int
	TotalConnections  int64
	MaxConnections    int
	TotalBytesSent    int64
	TotalBytesRecv    int64
	ConnectionsByNode map[string]int
}
