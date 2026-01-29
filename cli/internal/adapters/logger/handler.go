// Package logger implements a logging adapter using log/slog.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

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
		color = termenv.RGBColor(string(style.Mist))
	}

	// Build attribute string from handler attrs and record attrs
	attrParts := make([]string, 0, len(h.attrs)+r.NumAttrs())

	// Add handler-level attrs (already have group prefix applied)
	for _, attr := range h.attrs {
		attrParts = append(attrParts, attr.Key+"="+attr.Value.String())
	}

	// Add record-level attrs (apply current group prefix)
	r.Attrs(func(attr slog.Attr) bool {
		key := attr.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrParts = append(attrParts, key+"="+attr.Value.String())
		return true
	})

	if len(attrParts) > 0 {
		msg += " " + strings.Join(attrParts, " ")
	}

	styled := h.out.String(msg).Foreground(color)
	_, err := h.out.WriteString(styled.String() + "\n")

	return err
}

// WithAttrs returns a new Handler with the given attributes appended.
// Attributes are stored with the current group prefix already applied.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)

	// Apply current group prefix to new attrs before storing
	for i, attr := range attrs {
		key := attr.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		newAttrs[len(h.attrs)+i] = slog.Attr{Key: key, Value: attr.Value}
	}

	return &PrettyHandler{
		out:   h.out,
		level: h.level,
		attrs: newAttrs,
		group: h.group,
	}
}

// WithGroup returns a new Handler with the given group name.
// Groups nest: calling WithGroup("a").WithGroup("b") results in group "a.b".
// Per slog.Handler contract, WithGroup("") returns the receiver unchanged.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}

	return &PrettyHandler{
		out:   h.out,
		level: h.level,
		attrs: h.attrs,
		group: newGroup,
	}
}
