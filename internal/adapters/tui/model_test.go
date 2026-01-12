package tui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/telemetry"
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

func TestUpdate_InitTasks(t *testing.T) {
	m := tui.Model{}
	tasks := []string{"task1", "task2", "task3"}
	msg := telemetry.MsgInitTasks{Tasks: tasks}

	updatedModel, cmd := m.Update(msg)
	newM, ok := updatedModel.(tui.Model)
	require.True(t, ok)

	assert.Nil(t, cmd)
	assert.Len(t, newM.Tasks, len(tasks))
	assert.Len(t, newM.TaskMap, len(tasks))

	for i, task := range tasks {
		assert.Equal(t, task, newM.Tasks[i].Name)
		assert.Equal(t, tui.StatusPending, newM.Tasks[i].Status)
		assert.Same(t, &newM.Tasks[i], newM.TaskMap[task])
	}
}

func TestUpdate_Navigation(t *testing.T) {
	// Initialize directly with tasks
	m := tui.Model{
		Tasks: []tui.TaskNode{
			{Name: "task1"},
			{Name: "task2"},
			{Name: "task3"},
		},
		SelectedIdx: 0,
		FollowMode:  true,
		TaskMap:     make(map[string]*tui.TaskNode),
	}
	// Setup map pointers
	for i := range m.Tasks {
		m.TaskMap[m.Tasks[i].Name] = &m.Tasks[i]
	}

	// Case 1: Down
	// tea.KeyMsg matching "down"
	msgDown := tea.KeyMsg{Type: tea.KeyDown}

	updatedModel, _ := m.Update(msgDown)
	newM, ok := updatedModel.(tui.Model)
	require.True(t, ok)

	assert.Equal(t, 1, newM.SelectedIdx)
	assert.False(t, newM.FollowMode, "FollowMode should be false after navigation")

	// Case 2: Up
	msgUp := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = newM.Update(msgUp)
	newM, ok = updatedModel.(tui.Model)
	require.True(t, ok)

	assert.Equal(t, 0, newM.SelectedIdx)
	assert.False(t, newM.FollowMode)

	// Case 3: Activate Follow Mode (Escape)
	newM.FollowMode = false
	// Start a task to see if it jumps to running (as per logic: "jump to the currently running task if any")
	newM.Tasks[2].Status = tui.StatusRunning

	msgEsc := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = newM.Update(msgEsc)
	newM, ok = updatedModel.(tui.Model)
	require.True(t, ok)

	assert.True(t, newM.FollowMode)
	assert.Equal(t, 2, newM.SelectedIdx, "Should jump to running task index")
}

func TestUpdate_AutoFollow(t *testing.T) {
	// Setup
	tasks := []string{"task1", "task2", "task3"}
	m := tui.Model{}
	updatedModel, _ := m.Update(telemetry.MsgInitTasks{Tasks: tasks})
	m = updatedModel.(tui.Model)

	// Case 1: Follow Mode True
	m.FollowMode = true
	m.SelectedIdx = 0
	m.ActiveTaskName = "task1"

	// Send Start for task2
	msg := telemetry.MsgTaskStart{Name: "task2", SpanID: "span2"}
	updatedModel, _ = m.Update(msg)
	newM, ok := updatedModel.(tui.Model)
	require.True(t, ok)

	assert.Equal(t, "task2", newM.ActiveTaskName, "Should switch active task in follow mode")
	assert.Equal(t, 1, newM.SelectedIdx)
	assert.Equal(t, tui.StatusRunning, newM.TaskMap["task2"].Status)

	// Case 2: Follow Mode False
	newM.FollowMode = false
	// Simulate user selected task3 manually
	newM.SelectedIdx = 2
	newM.ActiveTaskName = "task3"

	// Send Start for task1
	msgStart := telemetry.MsgTaskStart{Name: "task1", SpanID: "span1"}
	updatedModel2, _ := newM.Update(msgStart)
	newM2, ok := updatedModel2.(tui.Model)
	require.True(t, ok)

	// Active task name should stay as "task3" because we are NOT following
	assert.Equal(t, "task3", newM2.ActiveTaskName, "Should NOT switch active task when not in follow mode")
	// SelectedIdx should NOT change
	assert.Equal(t, 2, newM2.SelectedIdx)
	// But status of task1 SHOULD update
	assert.Equal(t, tui.StatusRunning, newM2.TaskMap["task1"].Status)
}

func TestUpdate_Logs(t *testing.T) {
	// Setup
	m := tui.Model{
		Tasks: []tui.TaskNode{{Name: "task1", Status: tui.StatusRunning}},
		Viewport: viewport.Model{
			Width:  100,
			Height: 20,
		},
		ActiveTaskName: "task1",
		AutoScroll:     true,
		TaskMap:        make(map[string]*tui.TaskNode),
		SpanMap:        make(map[string]*tui.TaskNode),
	}
	m.TaskMap["task1"] = &m.Tasks[0]
	m.SpanMap["span1"] = &m.Tasks[0] // associate span1 with task1

	// Send Log
	logData := []byte("hello world")
	msg := telemetry.MsgTaskLog{SpanID: "span1", Data: logData}

	updatedModel, cmd := m.Update(msg)
	newM, ok := updatedModel.(tui.Model)
	require.True(t, ok)

	assert.Nil(t, cmd)
	assert.Equal(t, logData, newM.Tasks[0].Logs)
	assert.Contains(t, newM.Viewport.View(), "hello world")
}

func TestUpdate_WindowSize(t *testing.T) {
	m := tui.Model{
		Tasks:          []tui.TaskNode{{Name: "task1", Logs: []byte("some logs")}},
		ActiveTaskName: "task1",
		TaskMap:        make(map[string]*tui.TaskNode),
	}
	m.TaskMap["task1"] = &m.Tasks[0]
	m.Viewport.Width = 10
	m.Viewport.Height = 10

	// Send WindowSizeMsg
	// Width 100 -> List gets 30, Logs get 100 - 30 - 4 = 66
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(tui.Model)
	require.True(t, ok)

	// Check dimensions
	// listWidthRatio = 0.3
	assert.Equal(t, 48, newM.Viewport.Height) // 50 - 2
	assert.Equal(t, 66, newM.Viewport.Width)
}

func TestUpdate_TaskComplete(t *testing.T) {
	m := tui.Model{
		Tasks:   []tui.TaskNode{{Name: "task1", Status: tui.StatusRunning}},
		SpanMap: make(map[string]*tui.TaskNode),
	}
	m.SpanMap["span1"] = &m.Tasks[0]

	// Success case
	msgSuccess := telemetry.MsgTaskComplete{SpanID: "span1", Err: nil}
	updatedModel, _ := m.Update(msgSuccess)
	newM, ok := updatedModel.(tui.Model)
	require.True(t, ok)
	assert.Equal(t, tui.StatusDone, newM.Tasks[0].Status)

	// Error case
	// Reset status
	m.Tasks[0].Status = tui.StatusRunning
	msgError := telemetry.MsgTaskComplete{SpanID: "span1", Err: assert.AnError}
	updatedModel, _ = m.Update(msgError)
	newM, ok = updatedModel.(tui.Model)
	require.True(t, ok)
	assert.Equal(t, tui.StatusError, newM.Tasks[0].Status)
}
