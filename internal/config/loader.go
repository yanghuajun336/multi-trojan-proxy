package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// currentConfig holds the currently loaded configuration
var currentConfig *Config

// Load loads configuration from a YAML file
func Load(filePath string) (*Config, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load external rules file if specified
	if err := loadExternalRules(&cfg, filepath.Dir(filePath)); err != nil {
		return nil, err
	}

	// Set defaults
	cfg.SetDefaults()

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Store as current config
	currentConfig = &cfg

	return &cfg, nil
}

// GetCurrent returns the currently loaded configuration
func GetCurrent() *Config {
	return currentConfig
}

// Reload reloads the configuration from the same file
func Reload(filePath string) (*Config, error) {
	newConfig, err := Load(filePath)
	if err != nil {
		return nil, err
	}

	currentConfig = newConfig
	return newConfig, nil
}

// RulesConfig represents the external rules file structure
type RulesConfig struct {
	GeoIPDatabase string        `yaml:"geoip_database,omitempty"`
	Rules         []RoutingRule `yaml:"rules"`
}

// loadExternalRules loads routing rules from an external file if specified
func loadExternalRules(cfg *Config, configDir string) error {
	// Skip if no routing config or no rules file specified
	if cfg.Routing == nil || cfg.Routing.RulesFile == "" {
		return nil
	}

	// Resolve rules file path (relative to config directory)
	rulesPath := cfg.Routing.RulesFile
	if !filepath.IsAbs(rulesPath) {
		rulesPath = filepath.Join(configDir, rulesPath)
	}

	// Read rules file
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return fmt.Errorf("failed to read rules file %s: %w", rulesPath, err)
	}

	// Parse rules YAML
	var rulesConfig RulesConfig
	if err := yaml.Unmarshal(data, &rulesConfig); err != nil {
		return fmt.Errorf("failed to parse rules file %s: %w", rulesPath, err)
	}

	// Merge rules into main config
	cfg.Routing.Rules = rulesConfig.Rules

	// Use GeoIP database from rules file if not specified in main config
	if cfg.Routing.GeoIPDatabase == "" && rulesConfig.GeoIPDatabase != "" {
		cfg.Routing.GeoIPDatabase = rulesConfig.GeoIPDatabase
	}

	return nil
}
