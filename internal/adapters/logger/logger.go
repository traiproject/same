// Package logger implements a logging adapter using log/slog.
package logger

import (
	"log/slog"
	"os"

	"go.trai.ch/bob/internal/core/ports"
)

// Logger implements ports.Logger using log/slog.
type Logger struct {
	logger *slog.Logger
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

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.logger.Info(msg)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string) {
	l.logger.Warn(msg)
}

// Error logs an error message.
func (l *Logger) Error(err error) {
	l.logger.Error("operation failed", "error", err)
}
