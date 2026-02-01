package config

import (
	"fmt"
	"os"

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
