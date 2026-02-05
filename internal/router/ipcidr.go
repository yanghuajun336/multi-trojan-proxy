package router

import (
	"net"

	"github.com/yanghuajun/proxy/pkg/logger"
)

// IPCIDRMatcher IP-CIDR规则匹配器
type IPCIDRMatcher struct{}

// NewIPCIDRMatcher 创建IP-CIDR匹配器
func NewIPCIDRMatcher() *IPCIDRMatcher {
	return &IPCIDRMatcher{}
}

// MatchIPCIDR 匹配IP地址是否在CIDR范围内
func (m *IPCIDRMatcher) MatchIPCIDR(ipStr, cidr string) bool {
	// 解析CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		logger.Warn("invalid CIDR pattern",
			"cidr", cidr,
			"error", err)
		return false
	}

	// 解析IP地址
	ip := net.ParseIP(ipStr)
	if ip == nil {
		logger.Debug("invalid IP address",
			"ip", ipStr)
		return false
	}

	// 检查IP是否在CIDR范围内
	matched := ipNet.Contains(ip)

	if matched {
		logger.Debug("IP-CIDR rule matched",
			"ip", ipStr,
			"cidr", cidr)
	}

	return matched
}

// IsPrivateIP 检查是否为私有IP
func (m *IPCIDRMatcher) IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 检查常见的私有IP段
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // Link-local
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}

	for _, cidr := range privateRanges {
		if m.MatchIPCIDR(ipStr, cidr) {
			return true
		}
	}

	return false
}
