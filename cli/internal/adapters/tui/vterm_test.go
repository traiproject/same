package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestVterm_Write(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		initialOffset  int
		initialHeight  int
		input          string
		expectedOffset int // -1 means "maxOffset"
	}{
		{
			name:           "write at bottom sticks to bottom",
			initialOffset:  0,
			initialHeight:  5,
			input:          "line1\nline2\nline3\nline4\nline5\nline6",
			expectedOffset: -1, // Should be at max offset
		},
		{
			name:          "write while scrolled up stays scrolled",
			initialOffset: 0,
			initialHeight: 5,
			input:         "line1\nline2\nline3\nline4\nline5\nline6",
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vt := tui.NewVterm()
			vt.SetHeight(tt.initialHeight)

			if tt.name == "write while scrolled up stays scrolled" {
				// Pre-fill and scroll up
				_, _ = vt.Write([]byte("1\n2\n3\n4\n5\n6\n"))
				vt.Offset = 0 // Scroll to top
			}

			_, err := vt.Write([]byte(tt.input))
			require.NoError(t, err)

			if tt.expectedOffset == -1 {
				assert.Equal(t, vt.MaxOffset(), vt.Offset)
			} else if tt.name == "write while scrolled up stays scrolled" {
				assert.Equal(t, 0, vt.Offset)
			}
		})
	}
}

func TestVterm_SetHeight(t *testing.T) {
	t.Parallel()

	vt := tui.NewVterm()
	// Fill with 10 lines
	input := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10"
	_, _ = vt.Write([]byte(input))

	// Case 1: Set height, should stick to bottom if already at bottom
	vt.Offset = vt.MaxOffset()
	vt.SetHeight(5)
	assert.Equal(t, 5, vt.Height)
	assert.Equal(t, vt.MaxOffset(), vt.Offset)

	// Case 2: Set height while scrolled up, should clamp if needed
	vt.Offset = 0
	vt.SetHeight(2)
	assert.Equal(t, 2, vt.Height)
	assert.Equal(t, 0, vt.Offset)

	// Case 3: Set height > used height
	vt.SetHeight(20)
	assert.Equal(t, 20, vt.Height)
	assert.Equal(t, 0, vt.Offset)

	// Case 4: Zero/Negative height
	vt.SetHeight(0)
	assert.Equal(t, 1, vt.Height)
}

func TestVterm_SetWidth(t *testing.T) {
	t.Parallel()

	vt := tui.NewVterm()
	vt.Prefix = ">> "

	vt.SetWidth(10)
	assert.Equal(t, 10, vt.Width)

	// Internal terminal width should be 10 - len(">> ") = 7
	// We can't easily check private vt.Cols, but we can verify no panic

	vt.SetWidth(0)
	assert.Equal(t, 1, vt.Width)
}

func TestVterm_Update(t *testing.T) {
	t.Parallel()

	vt := tui.NewVterm()
	vt.SetHeight(2)
	// Fill with 4 lines: 0, 1, 2, 3
	_, _ = vt.Write([]byte("0\n1\n2\n3"))

	// Max offset should be 4 - 2 = 2
	// Expected view at max: lines 2, 3

	// Start at bottom
	vt.Offset = vt.MaxOffset()
	assert.Equal(t, 2, vt.Offset)

	// Key: up/k
	vt.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, vt.Offset)

	vt.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, vt.Offset)

	// Cap at 0
	vt.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, vt.Offset)

	// Key: down/j
	vt.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, vt.Offset)

	vt.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, vt.Offset)

	// Cap at max
	vt.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, vt.Offset)

	// Key: Home
	vt.Update(tea.KeyMsg{Type: tea.KeyHome})
	assert.Equal(t, 0, vt.Offset)

	// Key: End
	vt.Update(tea.KeyMsg{Type: tea.KeyEnd})
	assert.Equal(t, 2, vt.Offset)

	// Key: PgUp (Height=2)
	vt.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	assert.Equal(t, 0, vt.Offset)

	// Key: PgDown
	vt.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	assert.Equal(t, 2, vt.Offset)
}

func TestVterm_View(t *testing.T) {
	t.Parallel()

	vt := tui.NewVterm()
	vt.SetHeight(2)
	vt.Prefix = "> "

	// Write "hello\nworld"
	_, _ = vt.Write([]byte("hello\nworld"))

	// Expect:
	// > hello
	// > world

	// Strip ANSI codes for comparison
	stripAnsi := func(s string) string {
		return strings.ReplaceAll(s, "\x1b[0m", "")
	}
	output := vt.View()
	output = stripAnsi(output)

	expected := "> hello\n> world"
	assert.Equal(t, expected, output)

	// Verify lines
	lines := strings.Split(output, "\n")
	assert.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], "> "))
	assert.True(t, strings.HasPrefix(lines[1], "> "))
}
