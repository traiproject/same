// Package tui provides a textual user interface for the build system.
package tui

import "github.com/charmbracelet/bubbles/viewport"

// NewModel creates a new TUI model with default settings.
func NewModel() Model {
	return Model{
		Tasks:      make([]*TaskNode, 0),
		TaskMap:    make(map[string]*TaskNode),
		SpanMap:    make(map[string]*TaskNode),
		Viewport:   viewport.New(0, 0),
		AutoScroll: true,
	}
}
