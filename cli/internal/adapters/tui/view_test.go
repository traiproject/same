package tui_test

import (
	"testing"
	"time"

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
	assert.Contains(t, output, "[Running")

	// Case 3: Active task completed
	task.Status = tui.StatusDone
	output = m.View()
	assert.Contains(t, output, "LOGS: task1")
	assert.Contains(t, output, "[Took")
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

func TestView_EmptyTaskList(t *testing.T) {
	m := tui.Model{
		FlatList:   []*tui.TaskNode{},
		TreeRoots:  []*tui.TaskNode{},
		ListHeight: 10,
		ViewMode:   tui.ViewModeTree,
	}

	output := m.View()
	assert.Contains(t, output, "No tasks planned")
}

func TestView_TreeStructure(t *testing.T) {
	child1 := &tui.TaskNode{Name: "child1", Status: tui.StatusDone, Term: tui.NewVterm(), Depth: 1}
	child2 := &tui.TaskNode{Name: "child2", Status: tui.StatusPending, Term: tui.NewVterm(), Depth: 1}
	parent := &tui.TaskNode{
		Name:       "parent",
		Status:     tui.StatusRunning,
		Term:       tui.NewVterm(),
		Depth:      0,
		Children:   []*tui.TaskNode{child1, child2},
		IsExpanded: true,
	}
	child1.Parent = parent
	child2.Parent = parent

	m := tui.Model{
		FlatList:    []*tui.TaskNode{parent, child1, child2},
		TreeRoots:   []*tui.TaskNode{parent},
		ListHeight:  10,
		SelectedIdx: 0,
		TaskMap:     map[string]*tui.TaskNode{"parent": parent, "child1": child1, "child2": child2},
		ViewMode:    tui.ViewModeTree,
	}

	output := m.View()

	assert.Contains(t, output, "parent")
	assert.Contains(t, output, "child1")
	assert.Contains(t, output, "child2")
	assert.Contains(t, output, "▼")
	assert.Contains(t, output, "└──")
}

func TestView_DurationFormat(t *testing.T) {
	task := &tui.TaskNode{Name: "task1", Status: tui.StatusPending, Term: tui.NewVterm()}
	m := tui.Model{
		FlatList:   []*tui.TaskNode{task},
		TreeRoots:  []*tui.TaskNode{task},
		ListHeight: 10,
		ViewMode:   tui.ViewModeTree,
		TaskMap:    map[string]*tui.TaskNode{"task1": task},
	}

	output := m.View()
	assert.Contains(t, output, "[Pending]")

	task.Status = tui.StatusDone
	task.StartTime = task.StartTime.Add(-500 * time.Millisecond)
	output = m.View()
	assert.Contains(t, output, "[Took")
	assert.Contains(t, output, "ms")
}

func TestView_LogViewStatuses(t *testing.T) {
	tests := []struct {
		status   tui.TaskStatus
		expected string
	}{
		{tui.StatusPending, "[Pending]"},
		{tui.StatusError, "[Failed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			task := &tui.TaskNode{Name: "task1", Status: tt.status, Term: tui.NewVterm()}
			m := tui.Model{
				FlatList:       []*tui.TaskNode{task},
				ListHeight:     10,
				ViewMode:       tui.ViewModeLogs,
				ActiveTaskName: "task1",
				TaskMap:        map[string]*tui.TaskNode{"task1": task},
			}

			output := m.View()
			assert.Contains(t, output, tt.expected)
		})
	}
}

func TestView_LogViewTaskNotFound(t *testing.T) {
	m := tui.Model{
		FlatList:       []*tui.TaskNode{},
		ListHeight:     10,
		ViewMode:       tui.ViewModeLogs,
		ActiveTaskName: "nonexistent",
		TaskMap:        map[string]*tui.TaskNode{},
	}

	output := m.View()
	assert.Contains(t, output, "Task not found")
}

func TestView_DefaultViewMode(t *testing.T) {
	task := &tui.TaskNode{Name: "task1", Term: tui.NewVterm()}
	m := tui.Model{
		FlatList:   []*tui.TaskNode{task},
		TreeRoots:  []*tui.TaskNode{task},
		ListHeight: 10,
		ViewMode:   "invalid",
	}

	output := m.View()
	assert.Contains(t, output, "task1")
}

func TestView_FormatDuration_WithExecStartTime(t *testing.T) {
	now := time.Now()
	task := &tui.TaskNode{
		Name:          "task1",
		Status:        tui.StatusDone,
		Term:          tui.NewVterm(),
		StartTime:     now.Add(-2 * time.Second),
		ExecStartTime: now.Add(-1 * time.Second),
		EndTime:       now,
	}

	m := tui.Model{
		FlatList:   []*tui.TaskNode{task},
		TreeRoots:  []*tui.TaskNode{task},
		ListHeight: 10,
		ViewMode:   tui.ViewModeTree,
		TaskMap:    map[string]*tui.TaskNode{"task1": task},
	}

	output := m.View()

	assert.Contains(t, output, "[Took 1.0s]")
}

func TestView_FormatDuration_RunningTask(t *testing.T) {
	task := &tui.TaskNode{
		Name:      "task1",
		Status:    tui.StatusRunning,
		Term:      tui.NewVterm(),
		StartTime: time.Now().Add(-500 * time.Millisecond),
	}

	m := tui.Model{
		FlatList:   []*tui.TaskNode{task},
		TreeRoots:  []*tui.TaskNode{task},
		ListHeight: 10,
		ViewMode:   tui.ViewModeTree,
		TaskMap:    map[string]*tui.TaskNode{"task1": task},
	}

	output := m.View()

	assert.Contains(t, output, "[Running")
	assert.Contains(t, output, "ms")
}

func TestView_FullScreenLogView_WithDuration(t *testing.T) {
	now := time.Now()
	task := &tui.TaskNode{
		Name:      "task1",
		Status:    tui.StatusDone,
		Term:      tui.NewVterm(),
		StartTime: now.Add(-2 * time.Second),
		EndTime:   now,
	}

	m := tui.Model{
		FlatList:       []*tui.TaskNode{task},
		ListHeight:     10,
		ViewMode:       tui.ViewModeLogs,
		ActiveTaskName: "task1",
		TaskMap:        map[string]*tui.TaskNode{"task1": task},
	}

	output := m.View()

	assert.Contains(t, output, "LOGS: task1")
	assert.Contains(t, output, "[Took 2.0s]")
}

func TestFormatStatus_AllStates(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		task     *tui.TaskNode
		expected string
	}{
		{
			name: "Pending",
			task: &tui.TaskNode{
				Name:   "task1",
				Status: tui.StatusPending,
				Term:   tui.NewVterm(),
			},
			expected: "[Pending]",
		},
		{
			name: "Running",
			task: &tui.TaskNode{
				Name:      "task2",
				Status:    tui.StatusRunning,
				Term:      tui.NewVterm(),
				StartTime: now.Add(-1 * time.Second),
			},
			expected: "[Running",
		},
		{
			name: "Done",
			task: &tui.TaskNode{
				Name:      "task3",
				Status:    tui.StatusDone,
				Term:      tui.NewVterm(),
				StartTime: now.Add(-1 * time.Second),
				EndTime:   now,
			},
			expected: "[Took 1.0s]",
		},
		{
			name: "Cached",
			task: &tui.TaskNode{
				Name:      "task4",
				Status:    tui.StatusDone,
				Term:      tui.NewVterm(),
				StartTime: now.Add(-500 * time.Millisecond),
				EndTime:   now,
				Cached:    true,
			},
			expected: "[Cached",
		},
		{
			name: "Failed",
			task: &tui.TaskNode{
				Name:      "task5",
				Status:    tui.StatusError,
				Term:      tui.NewVterm(),
				StartTime: now.Add(-2 * time.Second),
				EndTime:   now,
			},
			expected: "[Failed 2.0s]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tui.Model{
				FlatList:   []*tui.TaskNode{tt.task},
				TreeRoots:  []*tui.TaskNode{tt.task},
				ListHeight: 10,
				ViewMode:   tui.ViewModeTree,
				TaskMap:    map[string]*tui.TaskNode{tt.task.Name: tt.task},
			}

			output := m.View()
			assert.Contains(t, output, tt.expected)
		})
	}
}

func TestCalculateRowNameWidth(t *testing.T) {
	tests := []struct {
		name     string
		task     *tui.TaskNode
		expected int
	}{
		{
			name: "Root level task",
			task: &tui.TaskNode{
				Name:  "root-task",
				Depth: 0,
			},
			expected: 2 + 1 + 1 + 9,
		},
		{
			name: "Depth 1 task",
			task: &tui.TaskNode{
				Name:  "child-task",
				Depth: 1,
			},
			expected: 2 + 4 + 2 + 1 + 1 + 10,
		},
		{
			name: "Depth 2 task",
			task: &tui.TaskNode{
				Name:  "grandchild",
				Depth: 2,
			},
			expected: 4 + 4 + 2 + 1 + 1 + 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := tui.CalculateRowNameWidth(tt.task)
			assert.Equal(t, tt.expected, width)
		})
	}
}

func TestCalculateMaxNameWidth(t *testing.T) {
	tasks := []*tui.TaskNode{
		{Name: "short", Depth: 0, Term: tui.NewVterm()},
		{Name: "very-long-task-name", Depth: 0, Term: tui.NewVterm()},
		{Name: "child", Depth: 1, Term: tui.NewVterm()},
	}

	m := tui.Model{
		FlatList:   tasks,
		TreeRoots:  tasks,
		ListHeight: 10,
		ViewMode:   tui.ViewModeTree,
	}

	maxWidth := m.CalculateMaxNameWidth(0, len(tasks))

	shortWidth := tui.CalculateRowNameWidth(tasks[0])
	longWidth := tui.CalculateRowNameWidth(tasks[1])
	childWidth := tui.CalculateRowNameWidth(tasks[2])

	expectedMax := longWidth
	if childWidth > expectedMax {
		expectedMax = childWidth
	}
	if shortWidth > expectedMax {
		expectedMax = shortWidth
	}

	assert.Equal(t, expectedMax, maxWidth)
}
