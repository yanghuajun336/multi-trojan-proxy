package config

import "fmt"

// ConfigError represents a configuration error
type ConfigError struct {
	message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: %s", e.message)
}

// ErrInvalidConfig creates a new configuration error
func ErrInvalidConfig(format string, args ...interface{}) error {
	return &ConfigError{
		message: fmt.Sprintf(format, args...),
	}
}
