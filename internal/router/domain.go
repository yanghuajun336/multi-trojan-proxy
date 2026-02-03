package router

import (
	"strings"

	"github.com/yanghuajun/proxy/pkg/logger"
)

// DomainMatcher 域名规则匹配器
type DomainMatcher struct{}

// NewDomainMatcher 创建域名匹配器
func NewDomainMatcher() *DomainMatcher {
	return &DomainMatcher{}
}

// MatchDomain 完全匹配域名
func (m *DomainMatcher) MatchDomain(domain, pattern string) bool {
	// 转换为小写进行比较
	domain = strings.ToLower(domain)
	pattern = strings.ToLower(pattern)

	// 移除端口号（如果有）
	if idx := strings.Index(domain, ":"); idx > 0 {
		domain = domain[:idx]
	}

	matched := domain == pattern
	if matched {
		logger.Debug("domain rule matched",
			"domain", domain,
			"pattern", pattern)
	}

	return matched
}

// MatchDomainSuffix 匹配域名后缀
func (m *DomainMatcher) MatchDomainSuffix(domain, pattern string) bool {
	// 转换为小写进行比较
	domain = strings.ToLower(domain)
	pattern = strings.ToLower(pattern)

	// 移除端口号（如果有）
	if idx := strings.Index(domain, ":"); idx > 0 {
		domain = domain[:idx]
	}

	// 精确匹配或后缀匹配
	// 例如: pattern="google.com" 应该匹配:
	// - "google.com" (精确)
	// - "www.google.com" (后缀)
	// - "mail.google.com" (后缀)
	matched := domain == pattern || strings.HasSuffix(domain, "."+pattern)

	if matched {
		logger.Debug("domain-suffix rule matched",
			"domain", domain,
			"pattern", pattern)
	}

	return matched
}
