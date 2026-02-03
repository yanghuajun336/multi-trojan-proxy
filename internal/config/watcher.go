package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/yanghuajun/proxy/pkg/logger"
)

// Watcher watches for configuration file changes
type Watcher struct {
	filePath string
	watcher  *fsnotify.Watcher
	onChange func(*Config) error
	stopCh   chan struct{}
}

// NewWatcher creates a new configuration file watcher
func NewWatcher(filePath string, onChange func(*Config) error) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch the directory containing the config file
	// (watching the file directly doesn't work well with editors that use atomic writes)
	dir := filepath.Dir(filePath)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	w := &Watcher{
		filePath: filePath,
		watcher:  watcher,
		onChange: onChange,
		stopCh:   make(chan struct{}),
	}

	return w, nil
}

// Start starts watching for file changes
func (w *Watcher) Start() {
	go w.watch()
}

// watch watches for file system events
func (w *Watcher) watch() {
	// Debounce timer to avoid multiple reloads for a single change
	var debounceTimer *time.Timer
	debounceDuration := 1 * time.Second

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			if filepath.Clean(event.Name) != filepath.Clean(w.filePath) {
				continue
			}

			// Only react to write and create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				logger.Info("config file changed: %s", event.Name)

				// Cancel existing timer if any
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				// Set new debounce timer
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					w.reload()
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			logger.Error("config watcher error: %v", err)

		case <-w.stopCh:
			return
		}
	}
}

// reload reloads the configuration and calls the onChange callback
func (w *Watcher) reload() {
	logger.Info("reloading configuration from %s", w.filePath)

	newConfig, err := Load(w.filePath)
	if err != nil {
		logger.Error("failed to reload config: %v", err)
		return
	}

	if w.onChange != nil {
		if err := w.onChange(newConfig); err != nil {
			logger.Error("failed to apply new config: %v", err)
			return
		}
	}

	logger.Info("configuration reloaded successfully")
}

// Stop stops watching for file changes
func (w *Watcher) Stop() error {
	close(w.stopCh)
	return w.watcher.Close()
}

// ConfigDiff 配置差异
type ConfigDiff struct {
	AddedNodes    []NodeConfig
	RemovedNodes  []NodeConfig
	ModifiedNodes []NodeConfig
	UnchangedNodes []NodeConfig
}

// CompareConfigs 比较新旧配置，计算差异
func CompareConfigs(oldCfg, newCfg *Config) *ConfigDiff {
	diff := &ConfigDiff{
		AddedNodes:    make([]NodeConfig, 0),
		RemovedNodes:  make([]NodeConfig, 0),
		ModifiedNodes: make([]NodeConfig, 0),
		UnchangedNodes: make([]NodeConfig, 0),
	}

	// 建立旧节点映射
	oldNodes := make(map[string]NodeConfig)
	for _, node := range oldCfg.Nodes {
		oldNodes[node.Name] = node
	}

	// 检查新节点
	for _, newNode := range newCfg.Nodes {
		if oldNode, exists := oldNodes[newNode.Name]; exists {
			// 节点存在，检查是否有修改
			if nodeModified(oldNode, newNode) {
				diff.ModifiedNodes = append(diff.ModifiedNodes, newNode)
			} else {
				diff.UnchangedNodes = append(diff.UnchangedNodes, newNode)
			}
			// 从旧节点映射中移除（剩余的就是被删除的）
			delete(oldNodes, newNode.Name)
		} else {
			// 新增节点
			diff.AddedNodes = append(diff.AddedNodes, newNode)
		}
	}

	// 剩余的旧节点都是被删除的
	for _, removedNode := range oldNodes {
		diff.RemovedNodes = append(diff.RemovedNodes, removedNode)
	}

	return diff
}

// nodeModified 检查节点配置是否有变化
func nodeModified(old, new NodeConfig) bool {
	return old.Server != new.Server ||
		old.Port != new.Port ||
		old.Password != new.Password ||
		old.Weight != new.Weight ||
		old.Enabled != new.Enabled
}
