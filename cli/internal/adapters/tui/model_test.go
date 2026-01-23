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

	// Create some dummy tasks
	tasks := []string{"task1", "task2"}
	msgInit := telemetry.MsgInitTasks{
		Tasks:        tasks,
		Targets:      tasks,
		Dependencies: map[string][]string{},
	}
	newM, _ := m.Update(msgInit)
	m = newM.(*tui.Model)

	// Send WindowSizeMsg
	width, height := 100, 50
	msgResize := tea.WindowSizeMsg{Width: width, Height: height}

	newM, _ = m.Update(msgResize)
	m = newM.(*tui.Model)

	// In tree view mode, log width should be full width
	assert.Equal(t, width, m.LogWidth)
	assert.Positive(t, m.LogHeight)
	assert.Positive(t, m.ListHeight)

	// Verify task terminals were resized
	for _, node := range m.TaskMap {
		assert.Equal(t, width, node.Term.Width)
		assert.Equal(t, m.LogHeight, node.Term.Height)
	}
}

func TestModel_Update_Navigation(t *testing.T) {
	t.Parallel()

	tasks := []*tui.TaskNode{
		{Name: "t1", Term: tui.NewVterm()},
		{Name: "t2", Term: tui.NewVterm()},
		{Name: "t3", Term: tui.NewVterm()},
	}

	m := &tui.Model{
		FlatList:   tasks,
		ListHeight: 2,
		ViewMode:   tui.ViewModeTree,
	}

	// 1. Initial State
	assert.Equal(t, 0, m.SelectedIdx)

	// 2. Down (j)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, m.SelectedIdx)
	assert.False(t, m.FollowMode)

	// 3. Down (down)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.SelectedIdx)

	// 4. Down at bottom (clamped)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.SelectedIdx)

	// 5. Up (k)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, m.SelectedIdx)

	// 6. Up (up)
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, m.SelectedIdx)

	// 7. Up at top (clamped)
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
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
	msgInit := telemetry.MsgInitTasks{
		Tasks:        tasks,
		Targets:      tasks,
		Dependencies: map[string][]string{},
	}
	m.Update(msgInit)

	assert.Len(t, m.FlatList, 1)
	assert.Contains(t, m.TaskMap, "task1")
	assert.Equal(t, tui.StatusPending, m.TaskMap["task1"].Status)
	assert.Equal(t, 100, m.TaskMap["task1"].Term.Width) // Should use pre-set dims

	// 2. Start Task
	spanID := "span-123"
	msgStart := telemetry.MsgTaskStart{Name: "task1", SpanID: spanID}
	m.Update(msgStart)

	assert.Equal(t, tui.StatusRunning, m.TaskMap["task1"].Status)
	assert.Contains(t, m.SpanMap, spanID)
	// Follow mode active -> should select this task
	assert.Equal(t, 0, m.SelectedIdx)
	assert.Equal(t, "task1", m.ActiveTaskName)

	// 3. Log Task
	msgLog := telemetry.MsgTaskLog{SpanID: spanID, Data: []byte("hello log")}
	m.Update(msgLog)

	output := m.TaskMap["task1"].Term.View()
	assert.Contains(t, output, "hello log")

	// 4. Complete Task (Success)
	msgComplete := telemetry.MsgTaskComplete{SpanID: spanID, Err: nil}
	m.Update(msgComplete)
	assert.Equal(t, tui.StatusDone, m.TaskMap["task1"].Status)

	// 5. Complete Task (Error)
	// Reset status for test
	m.TaskMap["task1"].Status = tui.StatusRunning
	msgError := telemetry.MsgTaskComplete{SpanID: spanID, Err: assert.AnError}
	m.Update(msgError)
	assert.Equal(t, tui.StatusError, m.TaskMap["task1"].Status)
}

func TestModel_Update_Esc(t *testing.T) {
	t.Parallel()

	tasks := []*tui.TaskNode{
		{Name: "t1", Status: tui.StatusDone, Term: tui.NewVterm()},
		{Name: "t2", Status: tui.StatusRunning, Term: tui.NewVterm()},
		{Name: "t3", Status: tui.StatusPending, Term: tui.NewVterm()},
	}

	m := &tui.Model{
		FlatList:    tasks,
		SelectedIdx: 0,
		FollowMode:  false,
		TaskMap: map[string]*tui.TaskNode{
			"t1": tasks[0],
			"t2": tasks[1],
			"t3": tasks[2],
		},
		ViewMode: tui.ViewModeTree,
	}

	// Press Esc
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// Should jump to running task (index 1) and enable follow mode
	assert.Equal(t, 1, m.SelectedIdx)
	assert.True(t, m.FollowMode)
	assert.Equal(t, "t2", m.ActiveTaskName)
}

func TestModel_Update_Quit(t *testing.T) {
	t.Parallel()
	m := &tui.Model{}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.Equal(t, tea.Quit(), cmd())

	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}
