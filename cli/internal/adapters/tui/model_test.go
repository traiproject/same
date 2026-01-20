package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestUpdate_InitTasks(t *testing.T) {
	m := &tui.Model{}
	tasks := []string{"task1", "task2", "task3"}
	msg := telemetry.MsgInitTasks{Tasks: tasks}

	updatedModel, cmd := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Nil(t, cmd)
	assert.Len(t, newM.Tasks, len(tasks))
	assert.Len(t, newM.TaskMap, len(tasks))

	for i, task := range tasks {
		assert.Equal(t, task, newM.Tasks[i].Name)
		assert.Equal(t, tui.StatusPending, newM.Tasks[i].Status)
		assert.Same(t, newM.Tasks[i], newM.TaskMap[task])
	}
}

func TestUpdate_Navigation(t *testing.T) {
	// Initialize directly with tasks
	m := &tui.Model{
		Tasks: []*tui.TaskNode{
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
		// Init term
		m.Tasks[i].Term = tui.NewVterm()
		m.TaskMap[m.Tasks[i].Name] = m.Tasks[i]
	}

	// Case 1: Down
	// tea.KeyMsg matching "down"
	msgDown := tea.KeyMsg{Type: tea.KeyDown}

	updatedModel, _ := m.Update(msgDown)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Equal(t, 1, newM.SelectedIdx)
	assert.False(t, newM.FollowMode, "FollowMode should be false after navigation")

	// Case 2: Up
	msgUp := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = newM.Update(msgUp)
	newM, ok = updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Equal(t, 0, newM.SelectedIdx)
	assert.False(t, newM.FollowMode)

	// Case 3: Activate Follow Mode (Escape)
	newM.FollowMode = false
	// Start a task to see if it jumps to running (as per logic: "jump to the currently running task if any")
	newM.Tasks[2].Status = tui.StatusRunning

	msgEsc := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ = newM.Update(msgEsc)
	newM, ok = updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.True(t, newM.FollowMode)
	assert.Equal(t, 2, newM.SelectedIdx, "Should jump to running task index")
}

func TestUpdate_AutoFollow(t *testing.T) {
	// Setup
	tasks := []string{"task1", "task2", "task3"}
	m := &tui.Model{}
	updatedModel, _ := m.Update(telemetry.MsgInitTasks{Tasks: tasks})
	m = updatedModel.(*tui.Model)

	// Case 1: Follow Mode True
	m.FollowMode = true
	m.SelectedIdx = 0
	m.ActiveTaskName = "task1"

	// Send Start for task2
	msg := telemetry.MsgTaskStart{Name: "task2", SpanID: "span2"}
	updatedModel, _ = m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
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
	newM2, ok := updatedModel2.(*tui.Model)
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
	m := &tui.Model{
		Tasks:          []*tui.TaskNode{{Name: "task1", Status: tui.StatusRunning, Term: tui.NewVterm()}},
		ActiveTaskName: "task1",
		AutoScroll:     true,
		TaskMap:        make(map[string]*tui.TaskNode),
		SpanMap:        make(map[string]*tui.TaskNode),
	}
	m.TaskMap["task1"] = m.Tasks[0]
	m.SpanMap["span1"] = m.Tasks[0] // associate span1 with task1

	// Determine width/height for term so it can render
	m.Tasks[0].Term.SetWidth(100)
	m.Tasks[0].Term.SetHeight(20)

	// Send Log
	logData := []byte("hello world")
	msg := telemetry.MsgTaskLog{SpanID: "span1", Data: logData}

	updatedModel, cmd := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	assert.Nil(t, cmd)
	// Check Term view contains data
	// Note: Vterm.View() returns the rendered string
	assert.Contains(t, newM.Tasks[0].Term.View(), "hello world")
}

func TestUpdate_WindowSize(t *testing.T) {
	m := &tui.Model{
		Tasks:          []*tui.TaskNode{{Name: "task1", Term: tui.NewVterm()}},
		ActiveTaskName: "task1",
		TaskMap:        make(map[string]*tui.TaskNode),
	}
	m.TaskMap["task1"] = m.Tasks[0]

	// Send WindowSizeMsg
	// Width 100 -> List gets 30, Logs get 100 - 30 - 4 = 66
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Check dimensions
	// listWidthRatio = 0.3
	// We can't easily check Term dimensions directly if they are private or just getters/Setters.
	// But we can check public properties if available.
	// Vterm struct has Height and Width public.
	assert.Equal(t, 66, newM.Tasks[0].Term.Width)
	// Height = 50 - 1 (header) = 49
	assert.Equal(t, 49, newM.Tasks[0].Term.Height)
}

func TestUpdate_TaskComplete(t *testing.T) {
	m := &tui.Model{
		Tasks:   []*tui.TaskNode{{Name: "task1", Status: tui.StatusRunning}},
		SpanMap: make(map[string]*tui.TaskNode),
	}
	m.SpanMap["span1"] = m.Tasks[0]

	// Success case
	msgSuccess := telemetry.MsgTaskComplete{SpanID: "span1", Err: nil}
	updatedModel, _ := m.Update(msgSuccess)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)
	assert.Equal(t, tui.StatusDone, newM.Tasks[0].Status)

	// Error case
	// Reset status
	m.Tasks[0].Status = tui.StatusRunning
	msgError := telemetry.MsgTaskComplete{SpanID: "span1", Err: assert.AnError}
	updatedModel, _ = m.Update(msgError)
	newM, ok = updatedModel.(*tui.Model)
	require.True(t, ok)
	assert.Equal(t, tui.StatusError, newM.Tasks[0].Status)
}

func TestInit(t *testing.T) {
	m := &tui.Model{}
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestUpdate_Quit(t *testing.T) {
	m := &tui.Model{}

	// Test "q"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.Equal(t, tea.Quit(), cmd())

	// Test "ctrl+c"
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}

func TestUpdate_Logs_InactiveTask(t *testing.T) {
	m := &tui.Model{
		Tasks:          []*tui.TaskNode{{Name: "task1", Term: tui.NewVterm()}, {Name: "task2", Term: tui.NewVterm()}},
		ActiveTaskName: "task1",
		SpanMap:        make(map[string]*tui.TaskNode),
	}
	m.SpanMap["span2"] = m.Tasks[1] // associate span2 with task2 (inactive)

	// Set some size
	m.Tasks[1].Term.SetWidth(50)
	m.Tasks[1].Term.SetHeight(10)

	msg := telemetry.MsgTaskLog{SpanID: "span2", Data: []byte("log for task 2")}

	updatedModel, _ := m.Update(msg)
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Logs for task2 should be updated
	// Check if view contains it
	assert.Contains(t, newM.Tasks[1].Term.View(), "log for task 2")
}

func TestUpdate_EmptyTasks_Esc(t *testing.T) {
	m := &tui.Model{
		Tasks: []*tui.TaskNode{},
	}

	// Press Esc with empty tasks
	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newM, ok := updatedModel.(*tui.Model)
	require.True(t, ok)

	// Should not panic, FollowMode should be true
	assert.True(t, newM.FollowMode)
}
