package logger_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/logger"
	"go.trai.ch/zerr"
)

// newTestLogger creates a logger with an injected bytes.Buffer for isolated testing.
// It also sets NO_COLOR=1 to ensure deterministic output without ANSI escape codes.
func newTestLogger(t *testing.T) (*logger.Logger, *bytes.Buffer) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")

	buf := &bytes.Buffer{}
	lg := logger.New().(*logger.Logger)
	lg.SetOutput(buf)
	return lg, buf
}

func TestLogger_Info(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		goldenName string
	}{
		{
			name:       "simple message",
			msg:        "some message",
			goldenName: "info_basic",
		},
		{
			name:       "empty message",
			msg:        "",
			goldenName: "info_empty",
		},
		{
			name:       "multiline message",
			msg:        "line1\nline2",
			goldenName: "info_multiline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg, buf := newTestLogger(t)
			lg.Info(tt.msg)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestLogger_Warn(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		goldenName string
	}{
		{
			name:       "simple warning",
			msg:        "some warning",
			goldenName: "warn_basic",
		},
		{
			name:       "empty warning",
			msg:        "",
			goldenName: "warn_empty",
		},
		{
			name:       "multiline warning",
			msg:        "warn1\nwarn2",
			goldenName: "warn_multiline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg, buf := newTestLogger(t)
			lg.Warn(tt.msg)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestLogger_Error(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		goldenName string
	}{
		{
			name:       "simple error",
			err:        os.ErrPermission,
			goldenName: "error_simple",
		},
		{
			name:       "not found error",
			err:        os.ErrNotExist,
			goldenName: "error_notfound",
		},
		{
			name:       "multiline error",
			err:        errors.New("yaml: unmarshal errors:\n  line 30: cannot unmarshal"),
			goldenName: "error_multiline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg, buf := newTestLogger(t)
			lg.Error(tt.err)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestLogger_Error_ZerrChain(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		goldenName string
	}{
		{
			name: "three level chain",
			err: zerr.Wrap(
				zerr.Wrap(
					errors.New("database connection failed"),
					"failed to load user data",
				),
				"failed to process request",
			),
			goldenName: "error_chain_zerr_three",
		},
		{
			name: "two level chain",
			err: zerr.Wrap(
				errors.New("underlying cause"),
				"wrapped message",
			),
			goldenName: "error_chain_zerr_two",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg, buf := newTestLogger(t)
			lg.Error(tt.err)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestLogger_Error_StdlibChain(t *testing.T) {
	// Standard errors using fmt.Errorf don't support chain traversal like zerr
	innerErr := errors.New("connection refused")
	middleErr := fmt.Errorf("failed to connect to database: %w", innerErr)
	outerErr := fmt.Errorf("failed to initialize service: %w", middleErr)

	lg, buf := newTestLogger(t)
	lg.Error(outerErr)

	g := goldie.New(t)
	g.Assert(t, "error_chain_stdlib", buf.Bytes())
}

func TestLogger_Error_WithMetadata(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		goldenName string
	}{
		{
			name: "single metadata field",
			err: zerr.With(
				zerr.New("task definition is empty"),
				"project", "cli",
			),
			goldenName: "error_metadata_single",
		},
		{
			name: "multiple metadata fields",
			err: func() error {
				e := zerr.New("task definition is empty")
				e = zerr.With(e, "project", "cli")
				e = zerr.With(e, "task", "try")
				return e
			}(),
			goldenName: "error_metadata_multi",
		},
		{
			name: "metadata on main error",
			err: func() error {
				inner := errors.New("connection refused")
				outer := zerr.Wrap(inner, "service unavailable")
				outer = zerr.With(outer, "service", "auth-api")
				outer = zerr.With(outer, "retry_count", 3)
				return outer
			}(),
			goldenName: "error_metadata_main",
		},
		{
			name: "partial metadata in chain",
			err: func() error {
				inner := zerr.With(zerr.New("database timeout"), "timeout_ms", 5000)
				middle := zerr.Wrap(inner, "failed to fetch user") // No metadata
				outer := zerr.With(middle, "user_id", "12345")
				return outer
			}(),
			goldenName: "error_metadata_partial",
		},
		{
			name: "sorted metadata keys",
			err: func() error {
				e := zerr.New("validation failed")
				e = zerr.With(e, "zebra", "z")
				e = zerr.With(e, "alpha", "a")
				e = zerr.With(e, "mike", "m")
				return e
			}(),
			goldenName: "error_metadata_sorted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg, buf := newTestLogger(t)
			lg.Error(tt.err)

			g := goldie.New(t)
			g.Assert(t, tt.goldenName, buf.Bytes())
		})
	}
}

func TestLogger_Error_Nil(t *testing.T) {
	lg, buf := newTestLogger(t)
	lg.Error(nil)

	assert.Empty(t, buf.String(), "Expected no output for nil error")
}

func TestLogger_SetJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonMode bool
		err      error
		wantJSON bool
	}{
		{
			name:     "JSON mode enabled",
			jsonMode: true,
			err:      errors.New("test error message"),
			wantJSON: true,
		},
		{
			name:     "JSON mode disabled",
			jsonMode: false,
			err:      errors.New("test error message"),
			wantJSON: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg, buf := newTestLogger(t)
			lg.SetJSON(tt.jsonMode)
			lg.Error(tt.err)

			output := buf.String()
			if tt.wantJSON {
				assert.Contains(t, output, `"error"`, "JSON output should contain error field")
				assert.Contains(t, output, `"level":"ERROR"`, "JSON output should contain level field")
				assert.NotContains(t, output, "✗", "JSON format should not have pretty markers")
			} else {
				g := goldie.New(t)
				g.Assert(t, "setjson_disabled", buf.Bytes())
			}
		})
	}
}

func TestLogger_SetJSON_WithErrorChain(t *testing.T) {
	innerErr := errors.New("database connection failed")
	middleErr := zerr.Wrap(innerErr, "failed to load user data")
	outerErr := zerr.With(middleErr, "user_id", "12345")

	lg, buf := newTestLogger(t)
	lg.SetJSON(true)
	lg.Error(outerErr)

	output := buf.String()

	// Verify JSON format contains expected fields without comparing exact timestamps
	assert.Contains(t, output, `"error"`, "JSON should contain error field")
	assert.Contains(t, output, `"level":"ERROR"`, "JSON should contain level field")
	assert.Contains(t, output, "failed to load user data", "JSON should contain error message")
	assert.Contains(t, output, "user_id", "JSON should contain metadata key")
	assert.Contains(t, output, "12345", "JSON should contain metadata value")
	assert.NotContains(t, output, "✗", "JSON format should not have pretty markers")
}

func TestLogger_FormatSwitching(t *testing.T) {
	lg, buf := newTestLogger(t)

	// Phase 1: Pretty format (default)
	err1 := errors.New("error in pretty mode")
	lg.Error(err1)
	prettyOutput := buf.String()
	buf.Reset()

	// Phase 2: Switch to JSON
	lg.SetJSON(true)
	err2 := errors.New("error in json mode")
	lg.Error(err2)
	jsonOutput := buf.String()
	buf.Reset()

	// Phase 3: Switch back to pretty
	lg.SetJSON(false)
	err3 := errors.New("error back in pretty mode")
	lg.Error(err3)
	backToPrettyOutput := buf.String()

	// Assertions
	assert.Contains(t, prettyOutput, "✗", "Pretty format should have error icon")
	assert.NotContains(t, prettyOutput, `"error"`, "Pretty format should not have JSON markers")

	assert.Contains(t, jsonOutput, `"error"`, "JSON format should have error field")
	assert.NotContains(t, jsonOutput, "✗", "JSON format should not have pretty markers")

	assert.Contains(t, backToPrettyOutput, "✗", "After switch back should have error icon")
	assert.NotContains(t, backToPrettyOutput, `"error"`, "After switch back should not have JSON markers")
}

func TestLogger_SetOutput(t *testing.T) {
	tests := []struct {
		name   string
		writer *bytes.Buffer
	}{
		{
			name:   "valid buffer",
			writer: &bytes.Buffer{},
		},
		{
			name:   "nil writer defaults to stderr",
			writer: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify no panic occurs
			require.NotPanics(t, func() {
				lg := logger.New().(*logger.Logger)
				lg.SetOutput(tt.writer)
			})
		})
	}
}

func TestLogger_New(t *testing.T) {
	lg := logger.New()
	require.NotNil(t, lg, "New() should return a non-nil logger")
}

// TestLogger_ConcurrentAccess tests thread-safety of the logger.
func TestLogger_ConcurrentAccess(t *testing.T) {
	lg, _ := newTestLogger(t)

	// Run concurrent operations
	done := make(chan bool, 6)

	go func() {
		lg.Info("concurrent info")
		done <- true
	}()
	go func() {
		lg.Warn("concurrent warn")
		done <- true
	}()
	go func() {
		lg.Error(errors.New("concurrent error"))
		done <- true
	}()
	go func() {
		lg.SetJSON(true)
		done <- true
	}()
	go func() {
		lg.SetJSON(false)
		done <- true
	}()
	go func() {
		buf := &bytes.Buffer{}
		lg.SetOutput(buf)
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 6; i++ {
		<-done
	}

	// If we get here without panic or deadlock, the test passes
}
