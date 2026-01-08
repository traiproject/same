package tui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/bob/internal/adapters/tui"
)

func TestWrapLog(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		// Use a function to verify result if simple equality isn't enough (e.g. exact wrapping points)
		verify   func(t *testing.T, input, got string, width int)
		expected string // use strict equality if verify is nil
	}{
		{
			name:  "no wrap needed",
			input: "hello world",
			width: 20,
			verify: func(t *testing.T, input, got string, width int) {
				t.Helper()
				// Expect it to contain the text
				assert.Contains(t, got, input)
				// Check max width of lines
				for _, line := range strings.Split(got, "\n") {
					assert.LessOrEqual(t, len(line), width, "line exceeds width")
				}
			},
		},
		{
			name:  "wrap needed",
			input: "hello world this is a long line",
			width: 10,
			verify: func(t *testing.T, input, got string, width int) {
				t.Helper()
				// Check that we have newlines (it wrapped)
				assert.Contains(t, got, "\n", "should produce newlines")
				// Check max width
				lines := strings.Split(got, "\n")
				for _, line := range lines {
					assert.LessOrEqual(t, len(line), width, "line exceeds width")
				}
				// Verify content is preserved (ignoring whitespace differences caused by wrapping)
				normalizedInput := strings.Join(strings.Fields(input), " ")
				normalizedGot := strings.Join(strings.Fields(got), " ")
				assert.Equal(t, normalizedInput, normalizedGot, "content mismatch")
			},
		},
		{
			name:     "width 0 (safety)",
			input:    "hello world",
			width:    0,
			expected: "hello world",
		},
		{
			name:     "negative width (safety)",
			input:    "hello world",
			width:    -5,
			expected: "hello world",
		},
		{
			name:     "empty input",
			input:    "",
			width:    10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tui.WrapLog(tt.input, tt.width)
			got = strings.ReplaceAll(got, "\r\n", "\n")

			if tt.verify != nil {
				tt.verify(t, tt.input, got, tt.width)
			} else {
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
