package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWithExternalRules tests loading configuration with external rules file
func TestLoadWithExternalRules(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create main config file
	mainConfig := `
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 10

nodes:
  - name: "test-node"
    server: "test.example.com"
    port: 443
    password: "test123"
    weight: 10
    enabled: true

routing:
  rules_file: "rules.yaml"
  geoip_database: "test.mmdb"

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: ""
  max_size: 100
`

	// Create rules file
	rulesConfig := `
rules:
  - type: DOMAIN-SUFFIX
    pattern: "example.com"
    action: DIRECT
  - type: DOMAIN-SUFFIX
    pattern: "google.com"
    action: PROXY
  - type: FINAL
    action: PROXY
`

	// Write files
	mainPath := filepath.Join(tmpDir, "config.yaml")
	rulesPath := filepath.Join(tmpDir, "rules.yaml")

	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatalf("Failed to write main config: %v", err)
	}

	if err := os.WriteFile(rulesPath, []byte(rulesConfig), 0644); err != nil {
		t.Fatalf("Failed to write rules config: %v", err)
	}

	// Load configuration
	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify routing rules were loaded
	if cfg.Routing == nil {
		t.Fatal("Routing config is nil")
	}

	if len(cfg.Routing.Rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(cfg.Routing.Rules))
	}

	// Verify first rule
	if cfg.Routing.Rules[0].Type != "DOMAIN-SUFFIX" {
		t.Errorf("Expected first rule type DOMAIN-SUFFIX, got %s", cfg.Routing.Rules[0].Type)
	}
	if cfg.Routing.Rules[0].Pattern != "example.com" {
		t.Errorf("Expected first rule pattern example.com, got %s", cfg.Routing.Rules[0].Pattern)
	}
	if cfg.Routing.Rules[0].Action != "DIRECT" {
		t.Errorf("Expected first rule action DIRECT, got %s", cfg.Routing.Rules[0].Action)
	}

	// Verify GeoIP database
	if cfg.Routing.GeoIPDatabase != "test.mmdb" {
		t.Errorf("Expected GeoIP database test.mmdb, got %s", cfg.Routing.GeoIPDatabase)
	}
}

// TestLoadWithExternalRulesGeoIP tests GeoIP database from rules file
func TestLoadWithExternalRulesGeoIP(t *testing.T) {
	tmpDir := t.TempDir()

	// Main config without GeoIP database
	mainConfig := `
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 10

nodes:
  - name: "test-node"
    server: "test.example.com"
    port: 443
    password: "test123"
    weight: 10
    enabled: true

routing:
  rules_file: "rules.yaml"

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: ""
  max_size: 100
`

	// Rules file with GeoIP database
	rulesConfig := `
geoip_database: "from-rules.mmdb"

rules:
  - type: GEOIP
    pattern: "CN"
    action: DIRECT
  - type: FINAL
    action: PROXY
`

	mainPath := filepath.Join(tmpDir, "config.yaml")
	rulesPath := filepath.Join(tmpDir, "rules.yaml")

	os.WriteFile(mainPath, []byte(mainConfig), 0644)
	os.WriteFile(rulesPath, []byte(rulesConfig), 0644)

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify GeoIP database from rules file
	if cfg.Routing.GeoIPDatabase != "from-rules.mmdb" {
		t.Errorf("Expected GeoIP database from-rules.mmdb, got %s", cfg.Routing.GeoIPDatabase)
	}
}

// TestLoadWithInlineRules tests backward compatibility with inline rules
func TestLoadWithInlineRules(t *testing.T) {
	tmpDir := t.TempDir()

	// Config with inline rules (no external file)
	mainConfig := `
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 10

nodes:
  - name: "test-node"
    server: "test.example.com"
    port: 443
    password: "test123"
    weight: 10
    enabled: true

routing:
  geoip_database: "inline.mmdb"
  rules:
    - type: DOMAIN-SUFFIX
      pattern: "inline.com"
      action: DIRECT
    - type: FINAL
      action: PROXY

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: ""
  max_size: 100
`

	mainPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(mainPath, []byte(mainConfig), 0644)

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify inline rules still work
	if len(cfg.Routing.Rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(cfg.Routing.Rules))
	}

	if cfg.Routing.Rules[0].Pattern != "inline.com" {
		t.Errorf("Expected inline.com, got %s", cfg.Routing.Rules[0].Pattern)
	}
}

// TestLoadWithMissingRulesFile tests error handling for missing rules file
func TestLoadWithMissingRulesFile(t *testing.T) {
	tmpDir := t.TempDir()

	mainConfig := `
proxy:
  listen: "0.0.0.0:8080"
  timeout: 30s
  max_concurrent: 10

nodes:
  - name: "test-node"
    server: "test.example.com"
    port: 443
    password: "test123"
    weight: 10
    enabled: true

routing:
  rules_file: "missing.yaml"

health_check:
  interval: 30s
  timeout: 5s
  failure_threshold: 3

logging:
  level: "info"
  file: ""
  max_size: 100
`

	mainPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(mainPath, []byte(mainConfig), 0644)

	_, err := Load(mainPath)
	if err == nil {
		t.Fatal("Expected error for missing rules file, got nil")
	}
}
