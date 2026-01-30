package logger_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/logger"
	"go.trai.ch/zerr"
)

func TestCollectErrorEntries(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantMessages []string
		wantMetadata []map[string]any
	}{
		{
			name:         "single standard error",
			err:          errors.New("simple error"),
			wantMessages: []string{"simple error"},
			wantMetadata: []map[string]any{nil},
		},
		{
			name: "zerr single error",
			err:  zerr.New("zerr error"),
			wantMessages: []string{
				"zerr error",
			},
			wantMetadata: []map[string]any{{}},
		},
		{
			name: "zerr wrapped chain",
			err: zerr.Wrap(
				zerr.Wrap(
					errors.New("root cause"),
					"middle layer",
				),
				"outer layer",
			),
			wantMessages: []string{
				"outer layer",
				"middle layer",
				"root cause",
			},
			wantMetadata: []map[string]any{{}, {}, nil},
		},
		{
			name: "zerr with metadata",
			err: zerr.With(
				zerr.With(
					zerr.New("base error"),
					"key1", "value1",
				),
				"key2", 42,
			),
			wantMessages: []string{"base error"},
			wantMetadata: []map[string]any{
				{"key1": "value1", "key2": 42},
			},
		},
		{
			name: "mixed chain with partial metadata",
			err: func() error {
				inner := zerr.With(zerr.New("inner"), "inner_key", "inner_val")
				outer := zerr.Wrap(inner, "outer")
				outer = zerr.With(outer, "outer_key", "outer_val")
				return outer
			}(),
			wantMessages: []string{"outer", "inner"},
			wantMetadata: []map[string]any{
				{"outer_key": "outer_val"},
				{"inner_key": "inner_val"},
			},
		},
		{
			name:         "nil error handling",
			err:          nil,
			wantMessages: nil,
			wantMetadata: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := logger.CollectErrorEntriesExported(tt.err)

			if tt.err == nil {
				assert.Empty(t, entries, "nil error should produce no entries")
				return
			}

			assert.Len(t, entries, len(tt.wantMessages), "entry count mismatch")
			assert.Len(t, tt.wantMetadata, len(tt.wantMessages), "metadata count mismatch")

			for i, wantMsg := range tt.wantMessages {
				assert.Equal(t, wantMsg, entries[i].Message, "message mismatch at index %d", i)
				assert.Equal(t, tt.wantMetadata[i], entries[i].Metadata, "metadata mismatch at index %d", i)
			}
		})
	}
}

func TestFormatErrorEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []logger.ErrorEntry
		want    string
	}{
		{
			name: "single entry",
			entries: []logger.ErrorEntry{
				{Message: "single error"},
			},
			want: "Error: single error",
		},
		{
			name: "two entries with caused by",
			entries: []logger.ErrorEntry{
				{Message: "outer error"},
				{Message: "inner error"},
			},
			want: "Error: outer error\n\n  Caused by:\n    → inner error",
		},
		{
			name: "three entries",
			entries: []logger.ErrorEntry{
				{Message: "first"},
				{Message: "second"},
				{Message: "third"},
			},
			want: "Error: first\n\n  Caused by:\n    → second\n    → third",
		},
		{
			name: "entry with metadata on main error",
			entries: []logger.ErrorEntry{
				{
					Message:  "main error",
					Metadata: map[string]any{"key": "value"},
				},
			},
			want: "Error: main error\n       key: value",
		},
		{
			name: "entry with metadata on cause",
			entries: []logger.ErrorEntry{
				{Message: "main"},
				{
					Message:  "cause",
					Metadata: map[string]any{"cause_key": "cause_val"},
				},
			},
			want: "Error: main\n\n  Caused by:\n    → cause\n      cause_key: cause_val",
		},
		{
			name: "multiline message",
			entries: []logger.ErrorEntry{
				{Message: "line1\nline2\nline3"},
			},
			want: "Error: line1\n       line2\n       line3",
		},
		{
			name: "multiline cause message",
			entries: []logger.ErrorEntry{
				{Message: "main"},
				{Message: "cause line1\ncause line2"},
			},
			want: "Error: main\n\n  Caused by:\n    → cause line1\n      cause line2",
		},
		{
			name:    "empty entries",
			entries: []logger.ErrorEntry{},
			want:    "",
		},
		{
			name: "metadata sorted alphabetically",
			entries: []logger.ErrorEntry{
				{
					Message: "error",
					Metadata: map[string]any{
						"zebra": "z",
						"alpha": "a",
						"mike":  "m",
					},
				},
			},
			want: "Error: error\n       alpha: a\n       mike: m\n       zebra: z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logger.FormatErrorEntriesExported(tt.entries)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCollectAndFormatIntegration(t *testing.T) {
	// Integration test that combines collect and format
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "zerr chain with metadata",
			err: func() error {
				inner := zerr.With(zerr.New("database timeout"), "timeout_ms", 5000)
				outer := zerr.Wrap(inner, "failed to fetch user")
				outer = zerr.With(outer, "user_id", "12345")
				return outer
			}(),
			want: "Error: failed to fetch user\n" +
				"       user_id: 12345\n\n" +
				"  Caused by:\n" +
				"    → database timeout\n" +
				"      timeout_ms: 5000",
		},
		{
			name: "simple standard error",
			err:  errors.New("simple"),
			want: "Error: simple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := logger.CollectErrorEntriesExported(tt.err)
			got := logger.FormatErrorEntriesExported(entries)
			assert.Equal(t, tt.want, got)
		})
	}
}
