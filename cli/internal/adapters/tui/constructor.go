// Package tui provides a textual user interface for the build system.
package tui

// NewModel creates a new TUI model with default settings.
func NewModel() Model {
	return Model{
		Tasks:      make([]*TaskNode, 0),
		TaskMap:    make(map[string]*TaskNode),
		SpanMap:    make(map[string]*TaskNode),
		AutoScroll: true,
	}
}
