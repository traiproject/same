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
	logger   *slog.Logger
	mu       sync.RWMutex
	jsonMode bool
	output   io.Writer
}

// New creates a new Logger instance.
func New() ports.Logger {
	handler := NewPrettyHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{
		logger: slog.New(handler),
		output: os.Stderr,
	}
}

// SetOutput updates the logger's output destination.
// This is thread-safe and updates the underlying slog handler.
// It preserves the current JSON mode setting.
// If w is nil, os.Stderr is used as the default.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if w == nil {
		w = os.Stderr
	}
	l.output = w

	var handler slog.Handler
	if l.jsonMode {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = NewPrettyHandler(w, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}
	l.logger = slog.New(handler)
}

// SetJSON switches between JSON and pretty logging.
// When enabled, logs are output as JSON. When disabled, pretty-printed logs are used.
// The output destination is preserved from SetOutput calls.
func (l *Logger) SetJSON(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.jsonMode = enable

	w := l.output
	if w == nil {
		w = os.Stderr
	}

	var handler slog.Handler
	if enable {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = NewPrettyHandler(w, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}
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
