package config

import "time"

// Config represents the entire proxy service configuration
type Config struct {
	Proxy       ProxyConfig       `yaml:"proxy"`
	Nodes       []NodeConfig      `yaml:"nodes"`
	Routing     *RoutingConfig    `yaml:"routing,omitempty"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	Logging     LoggingConfig     `yaml:"logging"`
}

// ProxyConfig represents the proxy service configuration
type ProxyConfig struct {
	Listen        string        `yaml:"listen"`
	Timeout       time.Duration `yaml:"timeout"`
	MaxConcurrent int           `yaml:"max_concurrent"`
}

// NodeConfig represents a Trojan node configuration
type NodeConfig struct {
	Name     string `yaml:"name"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	Weight   int    `yaml:"weight"`
	Enabled  bool   `yaml:"enabled"`
}

// RoutingConfig represents routing rules configuration
type RoutingConfig struct {
	GeoIPDatabase string         `yaml:"geoip_database"`
	Rules         []RoutingRule  `yaml:"rules"`
}

// RoutingRule represents a single routing rule
type RoutingRule struct {
	Type     string `yaml:"type"`     // DOMAIN, DOMAIN-SUFFIX, IP-CIDR, GEOIP, FINAL
	Pattern  string `yaml:"pattern"`  // Pattern to match
	Action   string `yaml:"action"`   // DIRECT, PROXY, REJECT
	Order    int    `yaml:"-"`        // Rule order (populated during loading)
	HitCount int64  `yaml:"-"`        // Number of times this rule was matched
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	FailureThreshold int           `yaml:"failure_threshold"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level   string `yaml:"level"`
	File    string `yaml:"file"`
	MaxSize int    `yaml:"max_size"`
}

// SetDefaults sets default values for the configuration
func (c *Config) SetDefaults() {
	// Proxy defaults
	if c.Proxy.Listen == "" {
		c.Proxy.Listen = "0.0.0.0:8080"
	}
	if c.Proxy.Timeout == 0 {
		c.Proxy.Timeout = 30 * time.Second
	}
	if c.Proxy.MaxConcurrent == 0 {
		c.Proxy.MaxConcurrent = 20
	}

	// Node defaults
	for i := range c.Nodes {
		if c.Nodes[i].Weight == 0 {
			c.Nodes[i].Weight = 10
		}
		// Enabled defaults to true if not explicitly set
		// This is handled by YAML unmarshaling
	}

	// Health check defaults
	if c.HealthCheck.Interval == 0 {
		c.HealthCheck.Interval = 30 * time.Second
	}
	if c.HealthCheck.Timeout == 0 {
		c.HealthCheck.Timeout = 5 * time.Second
	}
	if c.HealthCheck.FailureThreshold == 0 {
		c.HealthCheck.FailureThreshold = 3
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.MaxSize == 0 {
		c.Logging.MaxSize = 100
	}

	// Set routing rule order
	if c.Routing != nil {
		for i := range c.Routing.Rules {
			c.Routing.Rules[i].Order = i
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate proxy config
	if c.Proxy.Listen == "" {
		return ErrInvalidConfig("proxy.listen is required")
	}
	if c.Proxy.Timeout <= 0 {
		return ErrInvalidConfig("proxy.timeout must be positive")
	}
	if c.Proxy.MaxConcurrent <= 0 {
		return ErrInvalidConfig("proxy.max_concurrent must be positive")
	}

	// Validate nodes
	if len(c.Nodes) == 0 {
		return ErrInvalidConfig("at least one node is required")
	}

	nodeNames := make(map[string]bool)
	hasEnabledNode := false
	for i, node := range c.Nodes {
		if node.Name == "" {
			return ErrInvalidConfig("node[%d].name is required", i)
		}
		if nodeNames[node.Name] {
			return ErrInvalidConfig("duplicate node name: %s", node.Name)
		}
		nodeNames[node.Name] = true

		if node.Server == "" {
			return ErrInvalidConfig("node[%d].server is required", node.Name)
		}
		if node.Port <= 0 || node.Port > 65535 {
			return ErrInvalidConfig("node[%d].port must be between 1 and 65535", node.Name)
		}
		if node.Password == "" {
			return ErrInvalidConfig("node[%d].password is required", node.Name)
		}
		if node.Weight <= 0 {
			return ErrInvalidConfig("node[%d].weight must be positive", node.Name)
		}
		if node.Enabled {
			hasEnabledNode = true
		}
	}

	if !hasEnabledNode {
		return ErrInvalidConfig("at least one node must be enabled")
	}

	// Validate routing rules if present
	if c.Routing != nil && len(c.Routing.Rules) > 0 {
		if err := c.validateRoutingRules(); err != nil {
			return err
		}
	}

	// Validate health check config
	if c.HealthCheck.Interval <= 0 {
		return ErrInvalidConfig("health_check.interval must be positive")
	}
	if c.HealthCheck.Timeout <= 0 {
		return ErrInvalidConfig("health_check.timeout must be positive")
	}
	if c.HealthCheck.Timeout >= c.HealthCheck.Interval {
		return ErrInvalidConfig("health_check.timeout must be less than interval")
	}
	if c.HealthCheck.FailureThreshold <= 0 {
		return ErrInvalidConfig("health_check.failure_threshold must be positive")
	}

	// Validate logging config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return ErrInvalidConfig("logging.level must be one of: debug, info, warn, error")
	}
	if c.Logging.MaxSize <= 0 {
		return ErrInvalidConfig("logging.max_size must be positive")
	}

	return nil
}

// validateRoutingRules validates routing rules
func (c *Config) validateRoutingRules() error {
	validTypes := map[string]bool{
		"DOMAIN":        true,
		"DOMAIN-SUFFIX": true,
		"IP-CIDR":       true,
		"GEOIP":         true,
		"FINAL":         true,
	}

	validActions := map[string]bool{
		"DIRECT": true,
		"PROXY":  true,
		"REJECT": true,
	}

	hasFinal := false
	for i, rule := range c.Routing.Rules {
		// Validate type
		if !validTypes[rule.Type] {
			return ErrInvalidConfig("routing.rules[%d].type must be one of: DOMAIN, DOMAIN-SUFFIX, IP-CIDR, GEOIP, FINAL", i)
		}

		// Validate action
		if !validActions[rule.Action] {
			return ErrInvalidConfig("routing.rules[%d].action must be one of: DIRECT, PROXY, REJECT", i)
		}

		// Validate pattern
		if rule.Type != "FINAL" && rule.Pattern == "" {
			return ErrInvalidConfig("routing.rules[%d].pattern is required for type %s", i, rule.Type)
		}

		if rule.Type == "FINAL" {
			hasFinal = true
			if i != len(c.Routing.Rules)-1 {
				return ErrInvalidConfig("FINAL rule must be the last rule")
			}
		}

		// Check GEOIP database requirement
		if rule.Type == "GEOIP" && c.Routing.GeoIPDatabase == "" {
			return ErrInvalidConfig("routing.geoip_database is required when using GEOIP rules")
		}
	}

	if !hasFinal {
		return ErrInvalidConfig("routing rules must include a FINAL rule")
	}

	return nil
}
