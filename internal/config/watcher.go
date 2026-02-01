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
