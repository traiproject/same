// Package logger implements a logging adapter using log/slog.
package logger

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"go.trai.ch/same/internal/core/ports"
)

// messager describes an error that can report its own message without the chain.
// This matches the Message() method provided by zerr.Error (go.trai.ch/zerr v0.3.0+).
// If zerr's API changes, errors will gracefully fall back to standard error handling.
type messager interface {
	Message() string
}

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

	if err == nil {
		return
	}

	if l.jsonMode {
		l.logger.Error("operation failed", "error", err)
		return
	}

	// Collect messages by traversing the error chain programmatically
	var messages []string
	current := err

	for current != nil {
		if m, ok := current.(messager); ok {
			// zerr error: get raw message without chain
			messages = append(messages, m.Message())
			current = errors.Unwrap(current)
		} else {
			// Standard error: append full Error() and stop
			messages = append(messages, current.Error())
			break
		}
	}

	// Format the collected messages hierarchically
	var formattedLines []string

	for i, msg := range messages {
		lines := strings.Split(msg, "\n")

		if i == 0 {
			// Main error
			formattedLines = append(formattedLines, "Error: "+lines[0])
			// Indent any continuation lines to align with "Error: "
			for _, line := range lines[1:] {
				formattedLines = append(formattedLines, "       "+line)
			}
		} else {
			if i == 1 {
				// Add "Caused by:" header before first cause
				formattedLines = append(formattedLines, "", "  Caused by:")
			}
			// Add cause with arrow
			formattedLines = append(formattedLines, "    → "+lines[0])
			// Indent any continuation lines to align with the arrow
			for _, line := range lines[1:] {
				formattedLines = append(formattedLines, "      "+line)
			}
		}
	}

	msg := strings.Join(formattedLines, "\n")
	l.logger.Error(msg)
}
