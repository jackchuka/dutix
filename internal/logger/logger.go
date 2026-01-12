package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Logger represents a simple logger for dutix
type Logger struct {
	logger    *log.Logger
	debugMode bool
	logFile   *os.File
}

var globalLogger *Logger

const (
	maxLogSize = 10 * 1024 * 1024 // 10MB
	maxOldLogs = 3                // Keep 3 old log files
)

// Init initializes the global logger
func Init(debugMode bool) error {
	// Create log directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".dutix")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "dutix.log")

	// Rotate log if it's too large
	if err := rotateLogIfNeeded(logPath); err != nil {
		// Non-fatal error, just log to stderr
		fmt.Fprintf(os.Stderr, "Warning: Failed to rotate log: %v\n", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer (file + stderr in debug mode)
	var writer io.Writer
	if debugMode {
		writer = io.MultiWriter(logFile, os.Stderr)
	} else {
		writer = logFile
	}

	globalLogger = &Logger{
		logger:    log.New(writer, "", log.LstdFlags),
		debugMode: debugMode,
		logFile:   logFile,
	}

	Info("Dutix started", "debug_mode", debugMode)

	// Clean up old log files (older than 30 days)
	if err := CleanOldLogs(30); err != nil {
		Warn("Failed to clean old logs", "error", err)
	}

	return nil
}

// Close closes the logger
func Close() {
	if globalLogger != nil && globalLogger.logFile != nil {
		Info("Dutix shutting down")
		_ = globalLogger.logFile.Close()
		globalLogger.logFile = nil // Prevent double-close
	}
}

// Info logs an informational message
func Info(msg string, args ...any) {
	if globalLogger != nil {
		globalLogger.log("INFO", msg, args...)
	}
}

// Debug logs a debug message (only in debug mode)
func Debug(msg string, args ...any) {
	if globalLogger != nil && globalLogger.debugMode {
		globalLogger.log("DEBUG", msg, args...)
	}
}

// Error logs an error message
func Error(msg string, err error, args ...any) {
	if globalLogger != nil {
		allArgs := append([]any{"error", err}, args...)
		globalLogger.log("ERROR", msg, allArgs...)
	}
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	if globalLogger != nil {
		globalLogger.log("WARN", msg, args...)
	}
}

// log formats and writes a log message
func (l *Logger) log(level string, msg string, args ...any) {
	// Format additional arguments as key=value pairs
	var argsStr string
	if len(args) > 0 {
		argsStr = " "
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				argsStr += fmt.Sprintf("%v=%v ", args[i], args[i+1])
			} else {
				argsStr += fmt.Sprintf("%v=<missing> ", args[i])
			}
		}
	}

	l.logger.Printf("[%s] %s%s", level, msg, argsStr)
}

// rotateLogIfNeeded rotates the log file if it exceeds maxLogSize
func rotateLogIfNeeded(logPath string) error {
	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No log file yet, nothing to rotate
			return nil
		}
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	// Check if file size exceeds limit
	if info.Size() < maxLogSize {
		return nil
	}

	// Rotate: dutix.log -> dutix.log.1, dutix.log.1 -> dutix.log.2, etc.
	logDir := filepath.Dir(logPath)
	baseName := filepath.Base(logPath)

	// Shift existing rotated logs
	for i := maxOldLogs - 1; i >= 1; i-- {
		oldPath := filepath.Join(logDir, fmt.Sprintf("%s.%d", baseName, i))
		newPath := filepath.Join(logDir, fmt.Sprintf("%s.%d", baseName, i+1))

		// Delete the oldest if it exists
		if i == maxOldLogs-1 {
			_ = os.Remove(newPath)
		}

		// Rename if old file exists
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("failed to rotate %s: %w", oldPath, err)
			}
		}
	}

	// Rotate current log to .1
	rotatedPath := filepath.Join(logDir, fmt.Sprintf("%s.1", baseName))
	if err := os.Rename(logPath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rotate current log: %w", err)
	}

	return nil
}

// CleanOldLogs removes log files older than the specified number of days
func CleanOldLogs(days int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".dutix")
	cutoff := time.Now().AddDate(0, 0, -days)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only consider log files
		matched, err := filepath.Match("dutix.log*", entry.Name())
		if err != nil || !matched {
			continue
		}

		// Don't delete the current log file
		if entry.Name() == "dutix.log" {
			continue
		}

		filePath := filepath.Join(logDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Delete if older than cutoff
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filePath); err != nil {
				Warn("Failed to delete old log", "file", filePath, "error", err)
			} else {
				Info("Deleted old log file", "file", entry.Name())
			}
		}
	}

	return nil
}
