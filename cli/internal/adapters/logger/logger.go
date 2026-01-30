// Package logger implements a logging adapter using log/slog.
package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
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

// metadataer describes an error that can return structured metadata.
// This matches the Metadata() method provided by zerr.Error (go.trai.ch/zerr v0.3.0+).
type metadataer interface {
	Metadata() map[string]any
}

// errorEntry holds a message and its associated metadata for formatting.
type errorEntry struct {
	message  string
	metadata map[string]any
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

	entries := collectErrorEntries(err)
	msg := formatErrorEntries(entries)
	l.logger.Error(msg)
}

// collectErrorEntries traverses the error chain and extracts messages with metadata.
func collectErrorEntries(err error) []errorEntry {
	// Pre-allocate with typical error chain depth (3-5 levels)
	entries := make([]errorEntry, 0, 4) //nolint:mnd // typical error chain depth
	current := err

	for current != nil {
		entry := errorEntry{}

		// Extract metadata if available (check before processing message)
		if md, ok := current.(metadataer); ok {
			entry.metadata = md.Metadata()
		}

		if m, ok := current.(messager); ok {
			// zerr error: get raw message without chain
			entry.message = m.Message()
			current = errors.Unwrap(current)
		} else {
			// Standard error: use full Error() and stop
			entry.message = current.Error()
			current = nil
		}

		entries = append(entries, entry)
	}

	return entries
}

// formatErrorEntries formats the collected entries into a human-readable string.
func formatErrorEntries(entries []errorEntry) string {
	var formattedLines []string

	for i, entry := range entries {
		lines := strings.Split(entry.message, "\n")

		formattedLines = append(formattedLines, formatErrorMessage(i, lines)...)
		formattedLines = append(formattedLines, formatErrorMetadata(i, entry.metadata)...)
	}

	return strings.Join(formattedLines, "\n")
}

// formatErrorMessage formats the message lines for a single error entry.
func formatErrorMessage(index int, lines []string) []string {
	var result []string

	if index == 0 {
		// Main error
		result = append(result, "Error: "+lines[0])
		// Indent any continuation lines to align with "Error: "
		for _, line := range lines[1:] {
			result = append(result, "       "+line)
		}
	} else {
		if index == 1 {
			// Add "Caused by:" header before first cause
			result = append(result, "", "  Caused by:")
		}
		// Add cause with arrow
		result = append(result, "    → "+lines[0])
		// Indent any continuation lines to align with the arrow
		for _, line := range lines[1:] {
			result = append(result, "      "+line)
		}
	}

	return result
}

// formatErrorMetadata formats metadata fields for a single error entry.
func formatErrorMetadata(index int, metadata map[string]any) []string {
	if len(metadata) == 0 {
		return nil
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var result []string
	// Add each metadata field with appropriate indentation
	for _, k := range keys {
		v := metadata[k]
		if index == 0 {
			// Main error: indent to align with "Error: " (7 spaces)
			result = append(result, fmt.Sprintf("       %s: %v", k, v))
		} else {
			// Caused by: indent to align with arrow (6 spaces)
			result = append(result, fmt.Sprintf("      %s: %v", k, v))
		}
	}

	return result
}
