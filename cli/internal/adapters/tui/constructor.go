// Package tui provides a textual user interface for the build system.
package tui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// NewModel creates a new TUI model with default settings.
func NewModel(w io.Writer) Model {
	if w == nil {
		w = os.Stderr
	}

	out := NewOutput(w)
	lipgloss.SetColorProfile(out.Profile)

	return Model{
		Tasks:      make([]*TaskNode, 0),
		TaskMap:    make(map[string]*TaskNode),
		SpanMap:    make(map[string]*TaskNode),
		Output:     out,
		AutoScroll: true,
	}
}
