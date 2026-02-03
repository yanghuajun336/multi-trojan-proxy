package router

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/yanghuajun/proxy/internal/config"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Router 路由引擎
type Router struct {
	rules         []*RoutingRule
	domainMatcher *DomainMatcher
	ipMatcher     *IPCIDRMatcher
	geoipMatcher  *GeoIPMatcher
	dnsCache      *DNSCache
	mu            sync.RWMutex
}

// NewRouter 创建路由引擎
func NewRouter(rules []config.RoutingRule, geoipDBPath string) *Router {
	router := &Router{
		rules:         make([]*RoutingRule, 0, len(rules)),
		domainMatcher: NewDomainMatcher(),
		ipMatcher:     NewIPCIDRMatcher(),
		geoipMatcher:  NewGeoIPMatcher(geoipDBPath),
		dnsCache:      NewDNSCache(10000, 300), // 缓存10000个条目，5分钟过期
	}

	// 转换配置规则
	for i, rule := range rules {
		router.rules = append(router.rules, &RoutingRule{
			Type:     RuleType(rule.Type),
			Pattern:  rule.Pattern,
			Action:   Action(rule.Action),
			Order:    i,
			HitCount: 0,
		})
	}

	logger.Info("router initialized",
		"rules", len(router.rules),
		"geoip", router.geoipMatcher.IsEnabled())

	return router
}

// Route 路由决策
// 返回应该采取的动作（DIRECT/PROXY/REJECT）
func (r *Router) Route(ctx context.Context, target string) (Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 解析目标地址（可能是域名或IP:端口）
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		// 没有端口，直接使用target作为host
		host = target
	}

	// 检查是否为IP地址
	ip := net.ParseIP(host)
	isDomain := ip == nil

	// 遍历规则进行匹配
	for _, rule := range r.rules {
		matched := false

		switch rule.Type {
		case RuleTypeDomain:
			if isDomain {
				matched = r.domainMatcher.MatchDomain(host, rule.Pattern)
			}

		case RuleTypeDomainSuffix:
			if isDomain {
				matched = r.domainMatcher.MatchDomainSuffix(host, rule.Pattern)
			}

		case RuleTypeIPCIDR:
			// 如果是域名，需要先解析
			targetIP := host
			if isDomain {
				resolvedIP, err := r.dnsCache.Resolve(ctx, host)
				if err != nil {
					logger.Debug("DNS resolution failed for IP-CIDR rule",
						"domain", host,
						"error", err)
					continue
				}
				targetIP = resolvedIP
			}
			matched = r.ipMatcher.MatchIPCIDR(targetIP, rule.Pattern)

		case RuleTypeGEOIP:
			if !r.geoipMatcher.IsEnabled() {
				logger.Debug("GeoIP rule skipped, database not available")
				continue
			}

			// 如果是域名，需要先解析
			targetIP := host
			if isDomain {
				resolvedIP, err := r.dnsCache.Resolve(ctx, host)
				if err != nil {
					logger.Debug("DNS resolution failed for GeoIP rule",
						"domain", host,
						"error", err)
					continue
				}
				targetIP = resolvedIP
			}
			matched = r.geoipMatcher.MatchGeoIP(targetIP, rule.Pattern)

		case RuleTypeFinal:
			// FINAL规则总是匹配
			matched = true
		}

		if matched {
			// 更新命中计数
			atomic.AddInt64(&rule.HitCount, 1)

			logger.Debug("routing rule matched",
				"target", target,
				"rule", rule.Type,
				"pattern", rule.Pattern,
				"action", rule.Action)

			return rule.Action, nil
		}
	}

	// 如果没有任何规则匹配（理论上不应该发生，因为应该有FINAL规则）
	logger.Warn("no routing rule matched, falling back to PROXY",
		"target", target)
	return ActionProxy, nil
}

// UpdateRules 更新路由规则（用于配置重载）
func (r *Router) UpdateRules(rules []config.RoutingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	newRules := make([]*RoutingRule, 0, len(rules))
	for i, rule := range rules {
		newRules = append(newRules, &RoutingRule{
			Type:     RuleType(rule.Type),
			Pattern:  rule.Pattern,
			Action:   Action(rule.Action),
			Order:    i,
			HitCount: 0,
		})
	}

	r.rules = newRules
	logger.Info("routing rules updated", "count", len(newRules))
}

// GetStats 获取规则统计信息
func (r *Router) GetStats() []RuleStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make([]RuleStats, 0, len(r.rules))
	for _, rule := range r.rules {
		stats = append(stats, RuleStats{
			Type:     string(rule.Type),
			Pattern:  rule.Pattern,
			Action:   string(rule.Action),
			HitCount: atomic.LoadInt64(&rule.HitCount),
		})
	}

	return stats
}

// Close 关闭路由引擎
func (r *Router) Close() error {
	if r.geoipMatcher != nil {
		return r.geoipMatcher.Close()
	}
	return nil
}

// ValidateRules 验证路由规则配置
func ValidateRules(rules []config.RoutingRule) error {
	if len(rules) == 0 {
		return fmt.Errorf("no routing rules defined")
	}

	// 检查最后一条规则是否为FINAL
	lastRule := rules[len(rules)-1]
	if lastRule.Type != "FINAL" {
		return fmt.Errorf("last rule must be FINAL, got %s", lastRule.Type)
	}

	// 检查FINAL规则之前没有其他FINAL规则
	for i := 0; i < len(rules)-1; i++ {
		if rules[i].Type == "FINAL" {
			return fmt.Errorf("FINAL rule must be the last rule, found at position %d", i)
		}
	}

	// 验证每条规则
	for i, rule := range rules {
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("invalid rule at position %d: %w", i, err)
		}
	}

	return nil
}

// validateRule 验证单条规则
func validateRule(rule config.RoutingRule) error {
	// 验证规则类型
	switch rule.Type {
	case "DOMAIN", "DOMAIN-SUFFIX", "IP-CIDR", "GEOIP", "FINAL":
		// 有效类型
	default:
		return fmt.Errorf("invalid rule type: %s", rule.Type)
	}

	// 验证动作
	switch rule.Action {
	case "DIRECT", "PROXY", "REJECT":
		// 有效动作
	default:
		return fmt.Errorf("invalid action: %s", rule.Action)
	}

	// 验证Pattern（FINAL规则不需要Pattern）
	if rule.Type != "FINAL" && rule.Pattern == "" {
		return fmt.Errorf("pattern is required for rule type %s", rule.Type)
	}

	// IP-CIDR规则需要验证CIDR格式
	if rule.Type == "IP-CIDR" {
		_, _, err := net.ParseCIDR(rule.Pattern)
		if err != nil {
			return fmt.Errorf("invalid CIDR pattern: %s", rule.Pattern)
		}
	}

	return nil
}

// RuleStats 规则统计信息
type RuleStats struct {
	Type     string
	Pattern  string
	Action   string
	HitCount int64
}
