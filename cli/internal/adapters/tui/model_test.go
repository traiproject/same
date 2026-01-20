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
	m.Init() // explicit init call just for coverage

	// Create some dummy tasks
	tasks := []string{"task1", "task2"}
	msgInit := telemetry.MsgInitTasks{Tasks: tasks}
	newM, _ := m.Update(msgInit)
	m = newM.(*tui.Model)

	// Send WindowSizeMsg
	width, height := 100, 50
	msgResize := tea.WindowSizeMsg{Width: width, Height: height}

	newM, _ = m.Update(msgResize)
	m = newM.(*tui.Model)

	// Check dimensions
	// Check dimensions
	// taskListWidthRatio check manually or expose constant?
	expectedListWidth := int(float64(width) * 0.3)
	expectedLogWidth := width - expectedListWidth - 4 // subtracting logPaneBorderWidth (4)
	// We verify logic with hardcoded expectation based on known values.
	// 100 * 0.3 = 30. 100 - 30 - 4 = 66.

	assert.Equal(t, expectedLogWidth, m.LogWidth)
	assert.Positive(t, m.LogHeight)
	assert.Positive(t, m.ListHeight)

	// Verify task terminals were resized
	for _, node := range m.Tasks {
		assert.Equal(t, expectedLogWidth, node.Term.Width)
		assert.Equal(t, m.LogHeight, node.Term.Height)
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
	msgInit := telemetry.MsgInitTasks{Tasks: tasks}
	m.Update(msgInit)

	assert.Len(t, m.Tasks, 1)
	assert.Contains(t, m.TaskMap, "task1")
	assert.Equal(t, tui.StatusPending, m.Tasks[0].Status)
	assert.Equal(t, 100, m.Tasks[0].Term.Width) // Should use pre-set dims

	// 2. Start Task
	spanID := "span-123"
	msgStart := telemetry.MsgTaskStart{Name: "task1", SpanID: spanID}
	m.Update(msgStart)

	assert.Equal(t, tui.StatusRunning, m.Tasks[0].Status)
	assert.Contains(t, m.SpanMap, spanID)
	// Follow mode active -> should select this task
	assert.Equal(t, 0, m.SelectedIdx)
	assert.Equal(t, "task1", m.ActiveTaskName)

	// 3. Log Task
	msgLog := telemetry.MsgTaskLog{SpanID: spanID, Data: []byte("hello log")}
	m.Update(msgLog)

	output := m.Tasks[0].Term.View()
	assert.Contains(t, output, "hello log")

	// 4. Complete Task (Success)
	msgComplete := telemetry.MsgTaskComplete{SpanID: spanID, Err: nil}
	m.Update(msgComplete)
	assert.Equal(t, tui.StatusDone, m.Tasks[0].Status)

	// 5. Complete Task (Error)
	// Reset status for test
	m.Tasks[0].Status = tui.StatusRunning
	msgError := telemetry.MsgTaskComplete{SpanID: spanID, Err: assert.AnError}
	m.Update(msgError)
	assert.Equal(t, tui.StatusError, m.Tasks[0].Status)
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
	}
	// Setup map needed for updateActiveView
	m.TaskMap = map[string]*tui.TaskNode{
		"t1": m.Tasks[0],
		"t2": m.Tasks[1],
		"t3": m.Tasks[2],
	}
	m.Tasks[0].Term = tui.NewVterm()
	m.Tasks[1].Term = tui.NewVterm()
	m.Tasks[2].Term = tui.NewVterm()

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
