package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestView_Initialization(t *testing.T) {
	m := tui.Model{
		ListHeight: 0,
	}
	assert.Contains(t, m.View(), "Initializing...")
}

func TestView_TaskList(t *testing.T) {
	tasks := []*tui.TaskNode{
		{Name: "task1", Status: tui.StatusRunning, Term: tui.NewVterm()},
		{Name: "task2", Status: tui.StatusDone, Term: tui.NewVterm()},
		{Name: "task3", Status: tui.StatusError, Term: tui.NewVterm()},
		{Name: "task4", Status: tui.StatusPending, Term: tui.NewVterm()},
		{Name: "task5", Status: tui.StatusDone, Cached: true, Term: tui.NewVterm()},
	}

	m := tui.Model{
		FlatList:    tasks,
		TreeRoots:   tasks,
		ListHeight:  20,
		SelectedIdx: 0,
		TaskMap:     make(map[string]*tui.TaskNode),
		ViewMode:    tui.ViewModeTree,
	}
	for i := range tasks {
		m.TaskMap[tasks[i].Name] = tasks[i]
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
	// Case 1: No active task - use full screen log view
	task := &tui.TaskNode{Name: "task1", Term: tui.NewVterm()}
	m := tui.Model{
		FlatList:   []*tui.TaskNode{task},
		ListHeight: 20,
		ViewMode:   tui.ViewModeLogs,
		TaskMap:    map[string]*tui.TaskNode{"task1": task},
	}
	output := m.View()
	assert.Contains(t, output, "No task selected")

	// Case 2: Active task in full-screen log view
	m.ActiveTaskName = "task1"
	task.Status = tui.StatusRunning
	output = m.View()
	assert.Contains(t, output, "LOGS: task1")
	assert.Contains(t, output, "Running")

	// Case 3: Active task completed
	task.Status = tui.StatusDone
	output = m.View()
	assert.Contains(t, output, "LOGS: task1")
	assert.Contains(t, output, "Completed")
}

func TestView_LipglossIntegration(t *testing.T) {
	// Just ensure it renders something structure-wise
	task := &tui.TaskNode{Name: "task1", Term: tui.NewVterm()}
	m := tui.Model{
		FlatList:   []*tui.TaskNode{task},
		TreeRoots:  []*tui.TaskNode{task},
		ListHeight: 10,
		ViewMode:   tui.ViewModeTree,
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
