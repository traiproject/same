package tui_test

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestView_Initialization(t *testing.T) {
	m := tui.Model{
		Viewport: viewport.Model{Height: 0},
	}
	assert.Contains(t, m.View(), "Initializing...")
}

func TestView_TaskList(t *testing.T) {
	tasks := []*tui.TaskNode{
		{Name: "task1", Status: tui.StatusRunning},
		{Name: "task2", Status: tui.StatusDone},
		{Name: "task3", Status: tui.StatusError},
		{Name: "task4", Status: tui.StatusPending},
		{Name: "task5", Status: tui.StatusDone, Cached: true},
	}

	m := tui.Model{
		Tasks: tasks,
		Viewport: viewport.Model{
			Height: 20,
			Width:  100,
		},
		ListHeight:  20,
		SelectedIdx: 0,
		TaskMap:     make(map[string]*tui.TaskNode),
	}
	for i := range m.Tasks {
		m.TaskMap[m.Tasks[i].Name] = m.Tasks[i]
	}

	output := m.View()

	// Check for task names
	assert.Contains(t, output, "task1")
	assert.Contains(t, output, "task2")
	assert.Contains(t, output, "task3")
	assert.Contains(t, output, "task4")
	assert.Contains(t, output, "task5")

	// Check for icons (roughly)
	// Note: lipgloss might add escape codes, so distinct characters are better targets
	assert.Contains(t, output, "●") // Running
	assert.Contains(t, output, "✓") // Done
	assert.Contains(t, output, "✗") // Error
	assert.Contains(t, output, "○") // Pending
	assert.Contains(t, output, "⚡") // Cached

	// Check selection indicator
	// We expect task1 to have ">" and others to have "  "
	// Since Render adds styles, checking strictly is hard, but we can check if ">" is present near task1
	// For simplicity, just check that ">" exists.
	assert.Contains(t, output, ">")
}

func TestView_LogPane(t *testing.T) {
	// Case 1: No active task
	m := tui.Model{
		Viewport: viewport.Model{Height: 20, Width: 50},
	}
	output := m.View()
	assert.Contains(t, output, "LOGS (Waiting...)")

	// Case 2: Active task, FollowMode = true
	m.ActiveTaskName = "task1"
	m.FollowMode = true
	output = m.View()
	assert.Contains(t, output, "LOGS: task1 (Following)")

	// Case 3: Active task, FollowMode = false
	m.FollowMode = false
	output = m.View()
	assert.Contains(t, output, "LOGS: task1 (Manual)")
}

func TestView_LipglossIntegration(t *testing.T) {
	// Just ensure it renders something structure-wise
	m := tui.Model{
		Tasks: []*tui.TaskNode{{Name: "task1"}},
		Viewport: viewport.Model{
			Height: 10,
			Width:  40,
		},
		ListHeight: 10,
	}
	// Force some styles if possible, but mainly just ensuring no panic and non-empty
	output := m.View()
	assert.NotEmpty(t, output)

	// Check if it's joined horizontally (implies Side-by-Side)
	// If it joined, it likely contains ANSI codes or newlines arranged in a block
	// We can't easily assert the block layout without visual regression tools,
	// but we can assert we aren't crashing.

	// Let's verify that the output width is roughly what we expect or has newlines
	assert.Contains(t, output, "\n")
}
