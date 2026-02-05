package router

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/yanghuajun/proxy/pkg/logger"
)

// DNSCache DNS解析缓存
type DNSCache struct {
	cache      map[string]*cacheEntry
	maxEntries int
	ttl        time.Duration
	mu         sync.RWMutex
}

type cacheEntry struct {
	ip        string
	expiresAt time.Time
}

// NewDNSCache 创建DNS缓存
func NewDNSCache(maxEntries int, ttlSeconds int) *DNSCache {
	cache := &DNSCache{
		cache:      make(map[string]*cacheEntry),
		maxEntries: maxEntries,
		ttl:        time.Duration(ttlSeconds) * time.Second,
	}

	// 启动清理goroutine
	go cache.cleanupLoop()

	return cache
}

// Resolve 解析域名到IP（带缓存）
func (c *DNSCache) Resolve(ctx context.Context, domain string) (string, error) {
	// 检查缓存
	c.mu.RLock()
	entry, exists := c.cache[domain]
	c.mu.RUnlock()

	if exists && time.Now().Before(entry.expiresAt) {
		logger.Debug("DNS cache hit", "domain", domain, "ip", entry.ip)
		return entry.ip, nil
	}

	// 缓存未命中，执行DNS查询
	logger.Debug("DNS cache miss, resolving", "domain", domain)

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", domain)
	if err != nil {
		return "", err
	}

	if len(ips) == 0 {
		logger.Warn("no IP addresses found for domain", "domain", domain)
		return "", nil
	}

	// 使用第一个IP
	ip := ips[0].String()

	// 存入缓存
	c.mu.Lock()
	// 如果缓存已满，删除一些过期条目
	if len(c.cache) >= c.maxEntries {
		c.evictExpired()
	}

	c.cache[domain] = &cacheEntry{
		ip:        ip,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	logger.Debug("DNS resolved and cached",
		"domain", domain,
		"ip", ip,
		"ttl", c.ttl)

	return ip, nil
}

// evictExpired 删除过期条目（调用时必须持有写锁）
func (c *DNSCache) evictExpired() {
	now := time.Now()
	for domain, entry := range c.cache {
		if now.After(entry.expiresAt) {
			delete(c.cache, domain)
		}
	}
}

// cleanupLoop 定期清理过期条目
func (c *DNSCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		c.evictExpired()
		c.mu.Unlock()
	}
}

// Clear 清空缓存
func (c *DNSCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cacheEntry)
	logger.Info("DNS cache cleared")
}

// GetStats 获取缓存统计
func (c *DNSCache) GetStats() (entries int, capacity int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache), c.maxEntries
}
