package progress

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrorEntry represents an error log entry
type ErrorEntry struct {
	File      string
	Error     string
	Timestamp time.Time
}

// ErrorLogger logs errors to a file.
type ErrorLogger struct {
	mu      sync.Mutex
	logFile string
	errors  []ErrorEntry
	file    *os.File
}

// NewErrorLogger creates a new error logger.
func NewErrorLogger(logFile string) (*ErrorLogger, error) {
	logger := &ErrorLogger{
		logFile: logFile,
		errors:  []ErrorEntry{},
	}

	if logFile != "" {
		// Ensure parent directory exists
		dir := filepath.Dir(logFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("could not create log directory: %w", err)
		}

		// Open file for appending
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("could not open log file: %w", err)
		}
		logger.file = file
	}

	return logger, nil
}

// Log logs an error for a file.
func (l *ErrorLogger) Log(filePath, errorMsg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := ErrorEntry{
		File:      filePath,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}
	l.errors = append(l.errors, entry)

	if l.file != nil {
		line := fmt.Sprintf("%s | %s | %s\n",
			entry.Timestamp.Format(time.RFC3339),
			filepath.Base(filePath),
			errorMsg)
		l.file.WriteString(line)
	}
}

// Summary returns a summary of logged errors.
func (l *ErrorLogger) Summary() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.errors) == 0 {
		return "No errors"
	}
	return fmt.Sprintf("%d errors logged to %s", len(l.errors), l.logFile)
}

// ErrorCount returns the number of logged errors.
func (l *ErrorLogger) ErrorCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors)
}

// Close closes the log file.
func (l *ErrorLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
