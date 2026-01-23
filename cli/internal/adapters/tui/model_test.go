package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/adapters/tui"
)

func TestModel_Update_WindowSize(t *testing.T) {
	t.Parallel()

	m := &tui.Model{}
	m.Init()

	tasks := []string{"task1", "task2"}
	msgInit := telemetry.MsgInitTasks{Tasks: tasks}
	newM, _ := m.Update(msgInit)
	m = newM.(*tui.Model)

	width, height := 100, 50
	msgResize := tea.WindowSizeMsg{Width: width, Height: height}

	newM, _ = m.Update(msgResize)
	m = newM.(*tui.Model)

	// In tree view mode, LogWidth = msg.Width (full width)
	assert.Equal(t, width, m.LogWidth)
	assert.Positive(t, m.LogHeight)
	assert.Positive(t, m.ListHeight)

	// Verify task terminals were resized
	for name, node := range m.TaskMap {
		assert.Equal(t, width, node.Term.Width, "Task %s terminal width mismatch", name)
		assert.Equal(t, m.LogHeight, node.Term.Height, "Task %s terminal height mismatch", name)
	}
}

func TestModel_Update_Navigation(t *testing.T) {
	t.Parallel()

	m := &tui.Model{
		Tasks:      make([]*tui.TaskNode, 3),
		ListHeight: 2, // Small height to test scrolling
	}

	// Initialize tasks
	tags := []string{"t1", "t2", "t3"}
	for i, tag := range tags {
		m.Tasks[i] = &tui.TaskNode{Name: tag, Term: tui.NewVterm()}
	}
	m.FlatList = m.Tasks
	m.ViewMode = tui.ViewModeTree

	// 1. Initial State
	assert.Equal(t, 0, m.SelectedIdx)

	// 2. Down (j)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(*tui.Model)
	assert.Equal(t, 1, m.SelectedIdx)
	assert.False(t, m.FollowMode)

	// 3. Down (down)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*tui.Model)
	assert.Equal(t, 2, m.SelectedIdx)

	// 4. Down at bottom (clamped)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*tui.Model)
	assert.Equal(t, 2, m.SelectedIdx)

	// 5. Up (k)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(*tui.Model)
	assert.Equal(t, 1, m.SelectedIdx)

	// 6. Up (up)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(*tui.Model)
	assert.Equal(t, 0, m.SelectedIdx)

	// 7. Up at top (clamped)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(*tui.Model)
	assert.Equal(t, 0, m.SelectedIdx)
}

func TestModel_Update_Telemetry(t *testing.T) {
	t.Parallel()

	m := &tui.Model{
		LogWidth:   100,
		LogHeight:  50,
		FollowMode: true,
	}

	// 1. Init Tasks
	tasks := []string{"task1"}
	msgInit := telemetry.MsgInitTasks{Tasks: tasks}
	updated, _ := m.Update(msgInit)
	m = updated.(*tui.Model)

	assert.Len(t, m.TaskMap, 1)
	assert.Contains(t, m.TaskMap, "task1")
	task1 := m.TaskMap["task1"]
	assert.Equal(t, tui.StatusPending, task1.Status)
	assert.Equal(t, 100, task1.Term.Width)

	// 2. Start Task
	spanID := "span-123"
	msgStart := telemetry.MsgTaskStart{Name: "task1", SpanID: spanID}
	updated, _ = m.Update(msgStart)
	m = updated.(*tui.Model)

	task1 = m.TaskMap["task1"]
	assert.Equal(t, tui.StatusRunning, task1.Status)
	assert.Contains(t, m.SpanMap, spanID)
	assert.Equal(t, 0, m.SelectedIdx)
	assert.Equal(t, "task1", m.ActiveTaskName)

	// 3. Log Task
	msgLog := telemetry.MsgTaskLog{SpanID: spanID, Data: []byte("hello log")}
	updated, _ = m.Update(msgLog)
	m = updated.(*tui.Model)

	task1 = m.TaskMap["task1"]
	output := task1.Term.View()
	assert.Contains(t, output, "hello log")

	// 4. Complete Task (Success)
	msgComplete := telemetry.MsgTaskComplete{SpanID: spanID, Err: nil}
	updated, _ = m.Update(msgComplete)
	m = updated.(*tui.Model)
	
	task1 = m.TaskMap["task1"]
	assert.Equal(t, tui.StatusDone, task1.Status)

	// 5. Complete Task (Error)
	task1.Status = tui.StatusRunning
	msgError := telemetry.MsgTaskComplete{SpanID: spanID, Err: assert.AnError}
	updated, _ = m.Update(msgError)
	m = updated.(*tui.Model)
	
	task1 = m.TaskMap["task1"]
	assert.Equal(t, tui.StatusError, task1.Status)
}

func TestModel_Update_Esc(t *testing.T) {
	t.Parallel()

	m := &tui.Model{
		Tasks: []*tui.TaskNode{
			{Name: "t1", Status: tui.StatusDone},
			{Name: "t2", Status: tui.StatusRunning},
			{Name: "t3", Status: tui.StatusPending},
		},
		SelectedIdx: 0,
		FollowMode:  false,
		ViewMode:    tui.ViewModeTree,
	}
	m.FlatList = m.Tasks
	m.TaskMap = map[string]*tui.TaskNode{
		"t1": m.Tasks[0],
		"t2": m.Tasks[1],
		"t3": m.Tasks[2],
	}
	m.Tasks[0].Term = tui.NewVterm()
	m.Tasks[1].Term = tui.NewVterm()
	m.Tasks[2].Term = tui.NewVterm()
	for _, task := range m.Tasks {
		task.CanonicalNode = task
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*tui.Model)

	// Should jump to running task (index 1) and enable follow mode
	assert.Equal(t, 1, m.SelectedIdx)
	assert.True(t, m.FollowMode)
	// Note: ActiveTaskName is not set by Esc handler, only SelectedIdx
}

func TestModel_Update_Quit(t *testing.T) {
	t.Parallel()
	m := &tui.Model{}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.Equal(t, tea.Quit(), cmd())

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}
