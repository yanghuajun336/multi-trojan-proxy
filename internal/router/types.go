package router

// RuleType 路由规则类型
type RuleType string

const (
	RuleTypeDomain       RuleType = "DOMAIN"        // 域名完全匹配
	RuleTypeDomainSuffix RuleType = "DOMAIN-SUFFIX" // 域名后缀匹配
	RuleTypeIPCIDR       RuleType = "IP-CIDR"       // IP-CIDR范围匹配
	RuleTypeGEOIP        RuleType = "GEOIP"         // GeoIP国家代码匹配
	RuleTypeFinal        RuleType = "FINAL"         // 默认规则
)

// Action 路由动作
type Action string

const (
	ActionDirect Action = "DIRECT" // 直接连接，不使用代理
	ActionProxy  Action = "PROXY"  // 通过代理连接
	ActionReject Action = "REJECT" // 拒绝连接
)

// RoutingRule 路由规则
type RoutingRule struct {
	Type     RuleType // 规则类型
	Pattern  string   // 匹配模式
	Action   Action   // 路由动作
	Order    int      // 规则顺序
	HitCount int64    // 命中次数（统计用）
}

// MatchResult 匹配结果
type MatchResult struct {
	Matched bool         // 是否匹配
	Action  Action       // 路由动作
	Rule    *RoutingRule // 匹配的规则
}
