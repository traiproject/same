package logger_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/logger"
)

func TestPrettyHandler_Handle_Levels(t *testing.T) {
	tests := []struct {
		name       string
		level      slog.Level
		msg        string
		goldenName string
	}{
		{
			name:       "info level",
			level:      slog.LevelInfo,
			msg:        "information message",
			goldenName: "handler_info",
		},
		{
			name:       "warn level",
			level:      slog.LevelWarn,
			msg:        "warning message",
			goldenName: "handler_warn",
		},
		{
			name:       "error level",
			level:      slog.LevelError,
			msg:        "error message",
			goldenName: "handler_error",
		},
		{
			name:       "debug level filtered",
			level:      slog.LevelDebug,
			msg:        "debug message",
			goldenName: "handler_debug_filtered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")

			buf := &bytes.Buffer{}
			handler := logger.NewPrettyHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
			lg := slog.New(handler)

			lg.Log(t.Context(), tt.level, tt.msg)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestPrettyHandler_WithAttrs(t *testing.T) {
	tests := []struct {
		name       string
		attrs      []slog.Attr
		msg        string
		goldenName string
	}{
		{
			name:       "single attribute",
			attrs:      []slog.Attr{slog.String("key", "value")},
			msg:        "single attr message",
			goldenName: "handler_attrs_single",
		},
		{
			name:       "multiple attributes",
			attrs:      []slog.Attr{slog.String("a", "1"), slog.Int("b", 2)},
			msg:        "multi attr message",
			goldenName: "handler_attrs_multi",
		},
		{
			name:       "group attribute",
			attrs:      []slog.Attr{slog.Group("g", slog.String("k", "v"))},
			msg:        "group attr message",
			goldenName: "handler_attrs_group",
		},
		{
			name:       "nested group attribute",
			attrs:      []slog.Attr{slog.Group("outer", slog.Group("inner", slog.String("k", "v")))},
			msg:        "nested group message",
			goldenName: "handler_attrs_nested_group",
		},
		{
			name:       "mixed group and regular attrs",
			attrs:      []slog.Attr{slog.String("regular", "val"), slog.Group("g", slog.String("k", "v"))},
			msg:        "mixed attrs message",
			goldenName: "handler_attrs_mixed",
		},
		{
			name:       "empty attribute value",
			attrs:      []slog.Attr{slog.String("empty", "")},
			msg:        "empty value message",
			goldenName: "handler_attrs_empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")

			buf := &bytes.Buffer{}
			handler := logger.NewPrettyHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}).WithAttrs(tt.attrs)
			lg := slog.New(handler)

			lg.Info(tt.msg)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestPrettyHandler_WithGroup(t *testing.T) {
	tests := []struct {
		name       string
		groups     []string
		attrs      []slog.Attr
		msg        string
		goldenName string
	}{
		{
			name:       "single group",
			groups:     []string{"request"},
			attrs:      []slog.Attr{slog.String("id", "123")},
			msg:        "single group message",
			goldenName: "handler_group_single",
		},
		{
			name:       "nested groups",
			groups:     []string{"a", "b"},
			attrs:      []slog.Attr{slog.String("key", "val")},
			msg:        "nested group message",
			goldenName: "handler_group_nested",
		},
		{
			name:       "triple nested groups",
			groups:     []string{"a", "b", "c"},
			attrs:      []slog.Attr{slog.String("k", "v")},
			msg:        "triple nested message",
			goldenName: "handler_group_triple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")

			buf := &bytes.Buffer{}
			var handler slog.Handler = logger.NewPrettyHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})

			// Apply groups progressively
			for _, g := range tt.groups {
				handler = handler.WithGroup(g)
			}

			lg := slog.New(handler)
			lg.Info(tt.msg, tt.attrs[0].Key, tt.attrs[0].Value.Any())

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestPrettyHandler_WithGroup_EmptyName(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	buf := &bytes.Buffer{}
	handler := logger.NewPrettyHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	// WithGroup("") should return the same handler per slog contract
	sameHandler := handler.WithGroup("")

	lg := slog.New(sameHandler)
	lg.Info("empty group test", "key", "val")

	g := goldie.New(t)
	g.Assert(t, "handler_group_empty", buf.Bytes())
}

func TestPrettyHandler_Enabled(t *testing.T) {
	tests := []struct {
		name         string
		handlerLevel slog.Level
		recordLevel  slog.Level
		wantEnabled  bool
	}{
		{
			name:         "debug below info",
			handlerLevel: slog.LevelInfo,
			recordLevel:  slog.LevelDebug,
			wantEnabled:  false,
		},
		{
			name:         "info at info",
			handlerLevel: slog.LevelInfo,
			recordLevel:  slog.LevelInfo,
			wantEnabled:  true,
		},
		{
			name:         "warn above info",
			handlerLevel: slog.LevelInfo,
			recordLevel:  slog.LevelWarn,
			wantEnabled:  true,
		},
		{
			name:         "error above info",
			handlerLevel: slog.LevelInfo,
			recordLevel:  slog.LevelError,
			wantEnabled:  true,
		},
		{
			name:         "debug at debug",
			handlerLevel: slog.LevelDebug,
			recordLevel:  slog.LevelDebug,
			wantEnabled:  true,
		},
		{
			name:         "error at error",
			handlerLevel: slog.LevelError,
			recordLevel:  slog.LevelError,
			wantEnabled:  true,
		},
		{
			name:         "warn at error",
			handlerLevel: slog.LevelError,
			recordLevel:  slog.LevelWarn,
			wantEnabled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := logger.NewPrettyHandler(buf, &slog.HandlerOptions{
				Level: tt.handlerLevel,
			})

			ctx := t.Context()
			got := handler.Enabled(ctx, tt.recordLevel)
			assert.Equal(t, tt.wantEnabled, got)
		})
	}
}

func TestPrettyHandler_RecordAttrs(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		attrs      []any
		goldenName string
	}{
		{
			name:       "string attribute",
			msg:        "string attr",
			attrs:      []any{"key", "value"},
			goldenName: "handler_record_string",
		},
		{
			name:       "int attribute",
			msg:        "int attr",
			attrs:      []any{"count", 42},
			goldenName: "handler_record_int",
		},
		{
			name:       "bool attribute",
			msg:        "bool attr",
			attrs:      []any{"enabled", true},
			goldenName: "handler_record_bool",
		},
		{
			name:       "multiple attributes",
			msg:        "multiple attrs",
			attrs:      []any{"a", "1", "b", "2", "c", "3"},
			goldenName: "handler_record_multi",
		},
		{
			name:       "multiline message",
			msg:        "line1\nline2\nline3",
			attrs:      []any{},
			goldenName: "handler_record_multiline",
		},
		{
			name:       "empty message",
			msg:        "",
			attrs:      []any{"key", "value"},
			goldenName: "handler_record_empty_msg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")

			buf := &bytes.Buffer{}
			handler := logger.NewPrettyHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
			lg := slog.New(handler)

			lg.Info(tt.msg, tt.attrs...)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestPrettyHandler_Combination(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(h slog.Handler) slog.Handler
		msg        string
		attrs      []any
		goldenName string
	}{
		{
			name: "handler attrs with record attrs",
			setup: func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]slog.Attr{slog.String("hkey", "hval")})
			},
			msg:        "combined message",
			attrs:      []any{"rkey", "rval"},
			goldenName: "handler_combined_attrs",
		},
		{
			name: "group with handler and record attrs",
			setup: func(h slog.Handler) slog.Handler {
				return h.WithGroup("req").WithAttrs([]slog.Attr{slog.String("id", "123")})
			},
			msg:        "grouped message",
			attrs:      []any{"extra", "data"},
			goldenName: "handler_combined_group",
		},
		{
			name: "nested groups with attrs",
			setup: func(h slog.Handler) slog.Handler {
				return h.WithGroup("a").WithGroup("b").WithAttrs([]slog.Attr{slog.String("k", "v")})
			},
			msg:        "nested message",
			attrs:      []any{},
			goldenName: "handler_combined_nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")

			buf := &bytes.Buffer{}
			baseHandler := logger.NewPrettyHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})

			handler := tt.setup(baseHandler)
			lg := slog.New(handler)
			lg.Info(tt.msg, tt.attrs...)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestPrettyHandler_NilWriter(t *testing.T) {
	// Test that nil writer defaults to os.Stderr without panic
	require.NotPanics(t, func() {
		_ = logger.NewPrettyHandler(nil, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	})
}

func TestPrettyHandler_Handle_ReturnsError(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// Test with a writer that returns an error
	brokenWriter := &brokenWriter{}
	handler := logger.NewPrettyHandler(brokenWriter, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	lg := slog.New(handler)

	// This should not panic, even though write fails
	require.NotPanics(t, func() {
		lg.Info("this will fail to write")
	})
}

// brokenWriter simulates a writer that always returns an error.
type brokenWriter struct{}

func (bw *brokenWriter) Write([]byte) (int, error) {
	return 0, assert.AnError
}
