// Package tui provides a terminal user interface for the build system.
package tui

import (
	"io"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const defaultTickInterval = 100

// NewModel creates a new TUI model with default settings.
func NewModel(w io.Writer) Model {
	if w == nil {
		w = os.Stderr
	}

	out := NewOutput(w)
	lipgloss.SetColorProfile(out.Profile)

	return Model{
		Tasks:        make([]*TaskNode, 0),
		TaskMap:      make(map[string]*TaskNode),
		SpanMap:      make(map[string]*TaskNode),
		TreeRoots:    make([]*TaskNode, 0),
		FlatList:     make([]*TaskNode, 0),
		Output:       out,
		AutoScroll:   true,
		ViewMode:     ViewModeTree,
		FollowMode:   true,
		TickInterval: defaultTickInterval * time.Millisecond,
	}
}

// WithDisableTick disables the tick loop for testing.
//
//nolint:gocritic // hugeParam ignored
func (m Model) WithDisableTick() Model {
	m.DisableTick = true
	return m
}
