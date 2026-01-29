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
	handler := NewPrettyHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{
		logger: slog.New(handler),
	}
}

// SetOutput updates the logger's output destination.
// This is thread-safe and updates the underlying slog handler.
func (l *Logger) SetOutput(w io.Writer) {
	handler := NewPrettyHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
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
