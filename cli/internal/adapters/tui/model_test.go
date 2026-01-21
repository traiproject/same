package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/adapters/tui"
	"go.trai.ch/zerr"
)

func TestModel_Update(t *testing.T) {
	// Constants for testing
	const (
		taskName1 = "task-1"
		taskName2 = "task-2"
		taskName3 = "task-3"
		spanID1   = "span-1"
		spanID2   = "span-2"
	)
	initialTasks := []string{taskName1, taskName2, taskName3}

	// Helper to initialize a fresh model
	initModel := func(_ *testing.T) *tui.Model {
		m := &tui.Model{}
		// Send MsgInitTasks to set up the state
		initMsg := telemetry.MsgInitTasks{Tasks: initialTasks}
		updatedModel, _ := m.Update(initMsg)
		return updatedModel.(*tui.Model)
	}

	t.Run("Window Resizing", func(t *testing.T) {
		m := initModel(t)

		// Send WindowSizeMsg
		width, height := 100, 50
		msg := tea.WindowSizeMsg{Width: width, Height: height}
		updatedModel, _ := m.Update(msg)
		m = updatedModel.(*tui.Model)

		// Assertions based on constants in model.go:
		// taskListWidthRatio = 0.3
		// logPaneBorderWidth = 4
		expectedListWidth := int(float64(width) * 0.3)
		expectedLogWidth := width - expectedListWidth - 4

		assert.Equal(t, expectedLogWidth, m.LogWidth, "LogWidth calculation incorrect")
		assert.Equal(t, expectedLogWidth, m.Tasks[0].Term.Width, "Task term width not updated")

		// ListHeight depends on header rendering, so we just check it is reasonable
		assert.Positive(t, m.ListHeight, "ListHeight should be positive")
		assert.Less(t, m.ListHeight, height, "ListHeight should be less than total height")
		assert.Positive(t, m.LogHeight, "LogHeight should be positive")
		assert.Equal(t, m.LogHeight, m.Tasks[0].Term.Height, "Task term height not updated")
	})

	t.Run("Navigation & Keybindings", func(t *testing.T) {
		t.Run("Selection Navigation", func(t *testing.T) {
			m := initModel(t)
			m.SelectedIdx = 0

			// Move Down (j)
			m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
			assert.Equal(t, 1, m.SelectedIdx)
			assert.False(t, m.FollowMode, "FollowMode should be disabled on manual nav")

			// Move Down (down key)
			m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
			assert.Equal(t, 2, m.SelectedIdx)

			// Bounds check (end of list)
			m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
			assert.Equal(t, 2, m.SelectedIdx)

			// Move Up (k)
			m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
			assert.Equal(t, 1, m.SelectedIdx)

			// Move Up (up key)
			m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
			assert.Equal(t, 0, m.SelectedIdx)

			// Bounds check (start of list)
			m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyUp})
			assert.Equal(t, 0, m.SelectedIdx)
		})

		t.Run("Quit Commands", func(t *testing.T) {
			m := initModel(t)

			// q
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
			assert.Equal(t, tea.Quit(), cmd(), "q should return tea.Quit")

			// ctrl+c
			_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			assert.Equal(t, tea.Quit(), cmd(), "ctrl+c should return tea.Quit")
		})

		t.Run("Follow Mode (Esc)", func(t *testing.T) {
			m := initModel(t)

			// Start task 2 to have a running task
			m, _ = updateModel(m, telemetry.MsgTaskStart{Name: taskName2, SpanID: spanID1})

			// Move selection away manually
			m.SelectedIdx = 0
			m.FollowMode = false

			// Press Esc
			m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyEsc})

			assert.True(t, m.FollowMode, "Esc should enable FollowMode")
			assert.Equal(t, 1, m.SelectedIdx, "Esc should jump to running task (index 1)")
		})
	})

	t.Run("Telemetry Integration", func(t *testing.T) {
		t.Run("MsgInitTasks", func(t *testing.T) {
			m := &tui.Model{}
			msg := telemetry.MsgInitTasks{Tasks: []string{"A", "B"}}
			updatedModel, _ := m.Update(msg)
			m = updatedModel.(*tui.Model)

			assert.Len(t, m.Tasks, 2)
			assert.Len(t, m.TaskMap, 2)
			assert.Equal(t, "A", m.Tasks[0].Name)
			assert.Equal(t, tui.StatusPending, m.Tasks[0].Status)
		})

		t.Run("MsgTaskStart", func(t *testing.T) {
			m := initModel(t)

			msg := telemetry.MsgTaskStart{Name: taskName1, SpanID: spanID1}
			updatedModel, _ := m.Update(msg)
			m = updatedModel.(*tui.Model)

			requireTaskStatus(t, m, taskName1, tui.StatusRunning)
			assert.Equal(t, m.Tasks[0], m.SpanMap[spanID1], "SpanMap should map spanID")

			// Test FollowMode behavior
			// If FollowMode is true (default/initially? No, struct default is false. Wait, existing code doesn't set it in Init)
			// Actually model default bool is false.
			// Let's force FollowMode
			m.FollowMode = true
			msg2 := telemetry.MsgTaskStart{Name: taskName3, SpanID: spanID2}
			updatedModel, _ = m.Update(msg2)
			m = updatedModel.(*tui.Model)

			assert.Equal(t, 2, m.SelectedIdx, "FollowMode should switch selection to new task")
		})

		t.Run("MsgTaskLog", func(t *testing.T) {
			m := initModel(t)

			// Start task
			m, _ = updateModel(m, telemetry.MsgTaskStart{Name: taskName1, SpanID: spanID1})

			// Send Log
			logData := []byte("Hello World\n")
			msg := telemetry.MsgTaskLog{SpanID: spanID1, Data: logData}

			// We need to verify data is written. Vterm.UsedHeight() should increment if newlines are present?
			// Init Vterm dimensions first to ensure rendering happens or buffering works?
			// Vterm uses midterm.Terminal which defaults to 80x24 usually if not set?
			// Vterm uses NewAutoResizingTerminal.
			// Write should work regardless of size? Vterm.Write sticks to bottom.

			updatedModel, _ := m.Update(msg)
			m = updatedModel.(*tui.Model)

			node := m.SpanMap[spanID1]
			// Since we sent a newline, used height typically increases or we can check via some other way.
			// But specific assertion: "check term.UsedHeight() or internal buffer".
			// Let's assert UsedHeight > 0.
			assert.Positive(t, node.Term.UsedHeight(), "Term should have data")
		})

		t.Run("MsgTaskComplete", func(t *testing.T) {
			m := initModel(t)
			m, _ = updateModel(m, telemetry.MsgTaskStart{Name: taskName1, SpanID: spanID1})

			// Success
			msgSuccess := telemetry.MsgTaskComplete{SpanID: spanID1, Err: nil}
			m, _ = updateModel(m, msgSuccess)
			requireTaskStatus(t, m, taskName1, tui.StatusDone)

			// Error
			m, _ = updateModel(m, telemetry.MsgTaskStart{Name: taskName2, SpanID: spanID2})
			msgError := telemetry.MsgTaskComplete{SpanID: spanID2, Err: zerr.New("fail")}
			m, _ = updateModel(m, msgError)
			requireTaskStatus(t, m, taskName2, tui.StatusError)
		})
	})
}

// Helpers.

func updateModel(m *tui.Model, msg tea.Msg) (*tui.Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(*tui.Model), cmd
}

func requireTaskStatus(t *testing.T, m *tui.Model, taskName string, expected tui.TaskStatus) {
	t.Helper()
	node, ok := m.TaskMap[taskName]
	require.True(t, ok, "Task %s should exist in TaskMap", taskName)
	assert.Equal(t, expected, node.Status, "Task status map match")
}
