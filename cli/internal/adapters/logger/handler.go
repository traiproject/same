// Package logger implements a logging adapter using log/slog.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/muesli/termenv"
	"go.trai.ch/same/internal/ui/output"
	"go.trai.ch/same/internal/ui/style"
)

// PrettyHandler is a custom slog.Handler that produces human-readable,
// colored output using the shared UI components.
type PrettyHandler struct {
	out   *termenv.Output
	level slog.Leveler
	attrs []slog.Attr
	group string
}

// NewPrettyHandler creates a new PrettyHandler writing to the provided writer.
func NewPrettyHandler(w io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	if w == nil {
		w = os.Stderr
	}

	level := slog.LevelInfo
	if opts != nil && opts.Level != nil {
		level = opts.Level.Level()
	}

	levelVar := &slog.LevelVar{}
	levelVar.Set(level)

	return &PrettyHandler{
		out:   output.New(w),
		level: levelVar,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle formats and outputs the log record.
//
//nolint:gocritic // slog.Handler interface requires slog.Record by value
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	var msg string
	var color termenv.Color

	switch r.Level {
	case slog.LevelWarn:
		msg = style.Warning + " " + r.Message
		color = termenv.RGBColor(string(style.Yellow))
	case slog.LevelError:
		msg = style.Cross + " " + r.Message
		color = termenv.RGBColor(string(style.Red))
	default:
		msg = r.Message
		color = termenv.RGBColor(string(style.White))
	}

	styled := h.out.String(msg).Foreground(color)
	_, err := h.out.WriteString(styled.String() + "\n")

	return err
}

// WithAttrs returns a new Handler with the given attributes appended.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &PrettyHandler{
		out:   h.out,
		level: h.level,
		attrs: newAttrs,
		group: h.group,
	}
}

// WithGroup returns a new Handler with the given group name.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	return &PrettyHandler{
		out:   h.out,
		level: h.level,
		attrs: h.attrs,
		group: name,
	}
}
