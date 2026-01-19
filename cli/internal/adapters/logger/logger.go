// Package logger implements a logging adapter using log/slog.
package logger

import (
	"io"
	"log/slog"
	"os"
	"sync"

	"go.trai.ch/same/internal/core/ports"
)

// Logger implements ports.Logger using log/slog.
type Logger struct {
	logger *slog.Logger
	mu     sync.RWMutex
}

// New creates a new Logger instance.
func New() ports.Logger {
	// Use a text handler for human-readable output, writing to stderr as per 12-factor app guidelines
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{
		logger: slog.New(handler),
	}
}

// SetOutput updates the logger's output destination.
// This is thread-safe and updates the underlying slog handler.
func (l *Logger) SetOutput(w io.Writer) {
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	// We can't update slog.Logger in place safely if it's accessed concurrently
	// without a mutex.
	// However, we can replace the logger instance if we protect access.
	// But Logger methods use l.logger without lock.
	// To avoid locking on every log method (which is expensive), we can use atomic.Value
	// for the logger instance, OR we can accept that we need a lock.
	// Given this is a build tool, a RWMutex is fine.
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger = slog.New(handler)
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.Info(msg)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.Warn(msg)
}

// Error logs an error message.
func (l *Logger) Error(err error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.Error("operation failed", "error", err)
}
