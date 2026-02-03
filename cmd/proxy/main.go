package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yanghuajun/proxy/internal/config"
	"github.com/yanghuajun/proxy/internal/health"
	"github.com/yanghuajun/proxy/internal/proxy"
	"github.com/yanghuajun/proxy/internal/router"
	"github.com/yanghuajun/proxy/internal/trojan"
	"github.com/yanghuajun/proxy/pkg/logger"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
	version    = flag.Bool("version", false, "Print version information")
)

const (
	appVersion = "1.0.0"
	appName    = "Multi-Trojan Proxy"
)

func main() {
	flag.Parse()

	// Print version and exit
	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	// Load configuration
	fmt.Printf("Loading configuration from %s...\n", *configFile)
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.Init(cfg.Logging.Level, cfg.Logging.File, cfg.Logging.MaxSize); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("=== %s v%s ===", appName, appVersion)
	logger.Info("Configuration loaded from %s", *configFile)
	logger.Info("Loaded %d nodes", len(cfg.Nodes))

	// Count enabled nodes
	enabledCount := 0
	for _, node := range cfg.Nodes {
		if node.Enabled {
			enabledCount++
			logger.Info("Node: %s (%s:%d) - weight=%d", node.Name, node.Server, node.Port, node.Weight)
		}
	}
	logger.Info("%d nodes enabled", enabledCount)

	// Initialize health checker
	nodeConfigs := make([]*config.NodeConfig, len(cfg.Nodes))
	for i := range cfg.Nodes {
		nodeConfigs[i] = &cfg.Nodes[i]
	}
	healthChecker := health.NewChecker(nodeConfigs, cfg.HealthCheck)
	healthChecker.Start()
	defer healthChecker.Stop()

	// Initialize trojan node selector
	selector, err := trojan.NewSelector(cfg.Nodes)
	if err != nil {
		logger.Error("Failed to create node selector: %v", err)
		os.Exit(1)
	}
	defer selector.Close()

	// Connect health checker to selector
	selector.SetHealthChecker(healthChecker)

	// Create passive detector for request monitoring
	passiveDetector := health.NewPassiveDetector(healthChecker)

	// Initialize failover manager
	failover := proxy.NewFailover(selector, 3, passiveDetector)

	// Initialize proxy handler with failover support
	handler := proxy.NewHandler(selector, cfg.Proxy.Timeout)
	handler.SetFailover(failover)
	handler.SetPassiveDetector(passiveDetector)

	// Initialize router if routing rules are configured
	var rt *router.Router
	if cfg.Routing != nil && len(cfg.Routing.Rules) > 0 {
		// Validate routing rules
		if err := router.ValidateRules(cfg.Routing.Rules); err != nil {
			logger.Error("Invalid routing rules: %v", err)
			os.Exit(1)
		}

		geoipDBPath := ""
		if cfg.Routing != nil {
			geoipDBPath = cfg.Routing.GeoIPDatabase
		}

		rt = router.NewRouter(cfg.Routing.Rules, geoipDBPath)
		handler.SetRouter(rt)
		defer rt.Close()

		logger.Info("Router initialized with %d rules", len(cfg.Routing.Rules))
	} else {
		logger.Info("No routing rules configured, all traffic will use proxy")
	}

	// Create and start proxy server
	proxyServer := proxy.NewServer(cfg, handler)
	if err := proxyServer.Start(); err != nil {
		logger.Error("Failed to start proxy server: %v", err)
		os.Exit(1)
	}

	// Start configuration file watcher for hot reload
	configWatcher, err := config.NewWatcher(*configFile, func(newCfg *config.Config) error {
		logger.Info("applying new configuration...")

		// Calculate configuration difference
		diff := config.CompareConfigs(cfg, newCfg)

		logger.Info("configuration changes detected",
			"added", len(diff.AddedNodes),
			"removed", len(diff.RemovedNodes),
			"modified", len(diff.ModifiedNodes),
			"unchanged", len(diff.UnchangedNodes))

		// Log specific changes
		for _, node := range diff.AddedNodes {
			logger.Info("node added", "name", node.Name, "server", node.Server)
		}
		for _, node := range diff.RemovedNodes {
			logger.Info("node removed", "name", node.Name)
		}
		for _, node := range diff.ModifiedNodes {
			logger.Info("node modified", "name", node.Name)
		}

		// Update node selector with new nodes
		if err := selector.UpdateNodes(newCfg.Nodes); err != nil {
			return fmt.Errorf("failed to update node selector: %w", err)
		}

		// Update health checker with new nodes
		newNodeConfigs := make([]*config.NodeConfig, len(newCfg.Nodes))
		for i := range newCfg.Nodes {
			newNodeConfigs[i] = &newCfg.Nodes[i]
		}
		healthChecker.UpdateNodes(newNodeConfigs)

		// Update router if routing configuration changed
		if newCfg.Routing != nil && len(newCfg.Routing.Rules) > 0 && rt != nil {
			rt.UpdateRules(newCfg.Routing.Rules)
			logger.Info("routing rules updated", "count", len(newCfg.Routing.Rules))
		}

		// Update current config
		cfg = newCfg

		logger.Info("new configuration applied successfully")
		return nil
	})

	if err != nil {
		logger.Error("Failed to create config watcher: %v", err)
		os.Exit(1)
	}

	configWatcher.Start()
	defer configWatcher.Stop()

	logger.Info("configuration watcher started")

	logger.Info("=== Proxy service started successfully ===")
	logger.Info("Listening on %s", cfg.Proxy.Listen)
	logger.Info("Press Ctrl+C to stop")

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("Received signal %v, shutting down gracefully...", sig)

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop proxy server
	if err := proxyServer.Stop(ctx); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}

	// Stop health checker (already deferred)
	logger.Info("Stopping health checker...")

	// Stop config watcher (already deferred)
	logger.Info("Stopping config watcher...")

	logger.Info("Shutdown complete")
}
