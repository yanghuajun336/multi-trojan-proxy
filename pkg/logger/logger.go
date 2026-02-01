package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with levels and file rotation
type Logger struct {
	level      LogLevel
	file       *os.File
	logger     *log.Logger
	maxSize    int64 // Max file size in bytes
	currentSize int64
	mu         sync.Mutex
	logPath    string
}

var (
	// Global logger instance
	globalLogger *Logger
	once         sync.Once
)

// Init initializes the global logger
func Init(level string, filePath string, maxSizeMB int) error {
	var err error
	once.Do(func() {
		globalLogger, err = New(level, filePath, maxSizeMB)
	})
	return err
}

// New creates a new Logger instance
func New(level string, filePath string, maxSizeMB int) (*Logger, error) {
	logLevel := parseLogLevel(level)
	
	l := &Logger{
		level:   logLevel,
		maxSize: int64(maxSizeMB) * 1024 * 1024,
		logPath: filePath,
	}

	var writer io.Writer
	if filePath == "" {
		writer = os.Stdout
	} else {
		// Create log directory if it doesn't exist
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open log file
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		l.file = file
		writer = file

		// Get current file size
		if info, err := file.Stat(); err == nil {
			l.currentSize = info.Size()
		}
	}

	l.logger = log.New(writer, "", 0)
	return l, nil
}

// parseLogLevel converts string to LogLevel
func parseLogLevel(level string) LogLevel {
	switch level {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// log writes a log message with the specified level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if rotation is needed
	if l.file != nil && l.maxSize > 0 && l.currentSize >= l.maxSize {
		l.rotate()
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s %s\n", level.String(), timestamp, message)

	l.logger.Print(logLine)
	l.currentSize += int64(len(logLine))
}

// rotate rotates the log file
func (l *Logger) rotate() {
	if l.file == nil {
		return
	}

	// Close current file
	l.file.Close()

	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", l.logPath, timestamp)
	os.Rename(l.logPath, rotatedPath)

	// Open new file
	file, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to rotate log file: %v", err)
		return
	}

	l.file = file
	l.logger.SetOutput(file)
	l.currentSize = 0
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Close closes the logger and its file handle
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Global logger convenience functions
func Debug(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, args...)
	}
}

func Close() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}
