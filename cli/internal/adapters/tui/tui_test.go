package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/adapters/tui"
	"go.trai.ch/zerr"
)

func TestModel_Update(t *testing.T) {
	// Initialize model with some tasks
	const (
		taskName = "task-1"
		spanID   = "span-1"
	)
	initialTasks := []string{taskName, "task-2"}

	// Helper to initialize a fresh model
	initModel := func() *tui.Model {
		m := &tui.Model{}
		// Send MsgInitTasks to set up the state
		initMsg := telemetry.MsgInitTasks{Tasks: initialTasks}
		updatedModel, _ := m.Update(initMsg)
		return updatedModel.(*tui.Model)
	}

	t.Run("MsgTaskStart updates status to Running", func(t *testing.T) {
		m := initModel()

		// Verify initial state
		requireTaskStatus(t, m, taskName, tui.StatusPending)

		// Send MsgTaskStart
		startMsg := telemetry.MsgTaskStart{
			Name:   taskName,
			SpanID: spanID,
		}
		updatedModel, cmd := m.Update(startMsg)
		_ = cmd // ignore cmd
		m = updatedModel.(*tui.Model)

		// Verify status updated to Running
		requireTaskStatus(t, m, taskName, tui.StatusRunning)

		// Verify SpanMap is populated
		assert.Equal(t, m.Tasks[0], m.SpanMap[spanID], "SpanMap should map spanID to the correct TaskNode")
	})

	t.Run("MsgTaskComplete (Success) updates status to Done", func(t *testing.T) {
		m := initModel()

		// Pre-requisite: Start the task to populate SpanMap
		updatedModel, _ := m.Update(telemetry.MsgTaskStart{Name: taskName, SpanID: spanID})
		m = updatedModel.(*tui.Model)
		requireTaskStatus(t, m, taskName, tui.StatusRunning)

		// Send MsgTaskComplete (Success)
		completeMsg := telemetry.MsgTaskComplete{
			SpanID: spanID,
			Err:    nil,
		}
		updatedModel, _ = m.Update(completeMsg)
		m = updatedModel.(*tui.Model)

		// Verify status updated to Done
		requireTaskStatus(t, m, taskName, tui.StatusDone)
	})

	t.Run("MsgTaskComplete (Error) updates status to Error", func(t *testing.T) {
		m := initModel()

		// Pre-requisite: Start the task to populate SpanMap
		updatedModel, _ := m.Update(telemetry.MsgTaskStart{Name: taskName, SpanID: spanID})
		m = updatedModel.(*tui.Model)
		requireTaskStatus(t, m, taskName, tui.StatusRunning)

		// Send MsgTaskComplete (Error)
		completeMsg := telemetry.MsgTaskComplete{
			SpanID: spanID,
			Err:    zerr.New("something went wrong"),
		}
		updatedModel, _ = m.Update(completeMsg)
		m = updatedModel.(*tui.Model)

		// Verify status updated to Error
		requireTaskStatus(t, m, taskName, tui.StatusError)
	})
}

// Helper to check task status
// Note: m is passed as *tui.Model (pointer).
func requireTaskStatus(t *testing.T, m *tui.Model, taskName string, expected tui.TaskStatus) {
	t.Helper()
	node, ok := m.TaskMap[taskName]
	if !assert.True(t, ok, "Task %s should exist in TaskMap", taskName) {
		return
	}
	assert.Equal(t, expected, node.Status, "Task status for %s should be %s", taskName, expected)
}
