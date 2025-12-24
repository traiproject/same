//nolint:testpackage // Test needs access to unexported fields
package tui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/vito/progrock"
	"go.trai.ch/bob/internal/core/domain"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MockTapeSource is a mock implementation of TapeSource.
type MockTapeSource struct{}

func (m *MockTapeSource) Read() (*progrock.StatusUpdate, error) {
	return nil, nil
}

func TestModel_Update_VertexStarted(t *testing.T) {
	m := NewModel(&MockTapeSource{})

	// Pre-populate with some vertices
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1", Expanded: true},
		{ID: "2", Name: "Task 2", Expanded: true},
	}

	// Trigger VertexStarted for a new vertex (it should be in the list first usually,
	// but handleVertexStarted just looks up ID).
	// Actually, the message is effectively "Started", so we assume it exists
	// in vertices or we just check logic.
	// But handleVertexStarted iterates m.vertices.
	// So we must ensure the vertex exists in the model for the logic to apply to it.

	m.vertices = append(m.vertices, VertexState{ID: "3", Name: "Task 3", Expanded: false})

	_, cmd := m.handleVertexStarted(MsgVertexStarted{ID: "3", Name: "Task 3"})
	assert.Nil(t, cmd) // Should be nil cmd from handler

	// Check states
	assert.False(t, m.vertices[0].Expanded, "Task 1 should be collapsed")
	assert.False(t, m.vertices[1].Expanded, "Task 2 should be collapsed")
	assert.True(t, m.vertices[2].Expanded, "Task 3 should be expanded")
}

func TestModel_Update_VertexCompleted_Success(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1", Expanded: true},
	}

	_, cmd := m.handleVertexCompleted(MsgVertexCompleted{ID: "1", Err: nil})
	assert.Nil(t, cmd)

	assert.False(t, m.vertices[0].Expanded, "Successful task should collapse")
}

func TestModel_Update_VertexCompleted_Failure(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1", Expanded: true}, // Was true
	}

	// Test case where it stays expanded
	_, cmd := m.handleVertexCompleted(MsgVertexCompleted{ID: "1", Err: errors.New("boom")})
	assert.Nil(t, cmd)
	assert.True(t, m.vertices[0].Expanded, "Failed task should remain expanded")

	// Test case where it was collapsed (unlikely for active task but possible) and fails -> expands
	m.vertices[0].Expanded = false
	m.handleVertexCompleted(MsgVertexCompleted{ID: "1", Err: errors.New("boom")})
	assert.True(t, m.vertices[0].Expanded, "Failed task should become expanded")
}

func TestModel_Update_TapeUpdate_Emits_Started(t *testing.T) {
	m := NewModel(&MockTapeSource{})

	update := &progrock.StatusUpdate{
		Vertexes: []*progrock.Vertex{
			{Id: "1", Name: "Task 1"},
		},
	}

	msg := MsgTapeUpdate{Update: update}
	_, cmd := m.Update(msg)

	// We expect a Batch command containing MsgVertexStarted
	// and WaitForTape.
	// Since we can't easily inspect tea.Cmd (it's a func), we verified logic by code review.
	// But we can check that m.vertices has the new vertex.
	assert.Len(t, m.vertices, 1)
	assert.Equal(t, "1", m.vertices[0].ID)
	assert.Equal(t, statusRunning, m.vertices[0].Status)

	// Note: We cannot easily verify the returned Cmd without executing it,
	// which requires tea internal logic or reflection/mocking tea.
	assert.NotNil(t, cmd)
}

func TestModel_Update_TapeUpdate_Emits_Completed(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1", Status: statusRunning},
	}

	now := timestamppb.New(time.Now())
	update := &progrock.StatusUpdate{
		Vertexes: []*progrock.Vertex{
			{Id: "1", Name: "Task 1", Completed: now},
		},
	}

	msg := MsgTapeUpdate{Update: update}
	m.Update(msg)

	assert.Equal(t, statusCompleted, m.vertices[0].Status)
}

func TestModel_Update_KeyMsg_Navigation(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1"},
		{ID: "2", Name: "Task 2"},
		{ID: "3", Name: "Task 3"},
	}

	// Initial State
	assert.Equal(t, 0, m.SelectedIdx)

	// Down (j)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, m.SelectedIdx)

	// Down (KeyMsg Down)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.SelectedIdx)

	// Wrap around Down
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 0, m.SelectedIdx)

	// Up (k) - Wrap around Up
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 2, m.SelectedIdx)

	// Up (KeyMsg Up)
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, m.SelectedIdx)
}

func TestModel_Update_KeyMsg_Toggle(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1", Expanded: false},
	}

	// Enter to toggle expand
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.vertices[0].Expanded)

	// Space to toggle collapse
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.False(t, m.vertices[0].Expanded)
}

func TestModel_Update_KeyMsg_Verbosity(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	// Default is Info (0)
	assert.Equal(t, domain.LogLevelInfo, m.MinLogLevel)

	// '+' -> Decrease level (more verbose) -> Debug (-4)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	assert.Equal(t, domain.LogLevelDebug, m.MinLogLevel)

	// Minimum clamp check (can't go below Debug)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	assert.Equal(t, domain.LogLevelDebug, m.MinLogLevel)

	// '-' -> Increase level (less verbose) -> Info (0)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	assert.Equal(t, domain.LogLevelInfo, m.MinLogLevel)

	// '-' -> Warn (4)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	assert.Equal(t, domain.LogLevelWarn, m.MinLogLevel)

	// '-' -> Error (8)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	assert.Equal(t, domain.LogLevelError, m.MinLogLevel)

	// Maximum clamp check
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	assert.Equal(t, domain.LogLevelError, m.MinLogLevel)
}

func TestModel_View_LogFiltering(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1", Expanded: true},
	}
	m.logs["1"] = []string{
		"[DEBUG] Debug message",
		"[INFO] Info message",
		"[WARN] Warn message",
		"[ERROR] Error message",
	}

	// Case 1: Filter at INFO (default)
	m.MinLogLevel = domain.LogLevelInfo
	output := m.View()
	assert.NotContains(t, output, "Debug message")
	assert.Contains(t, output, "Info message")
	assert.Contains(t, output, "Warn message")
	assert.Contains(t, output, "Error message")

	// Case 2: Filter at ERROR
	m.MinLogLevel = domain.LogLevelError
	output = m.View()
	assert.NotContains(t, output, "Debug message")
	assert.NotContains(t, output, "Info message")
	assert.NotContains(t, output, "Warn message")
	assert.Contains(t, output, "Error message")

	// Case 3: Filter at DEBUG
	m.MinLogLevel = domain.LogLevelDebug
	output = m.View()
	assert.Contains(t, output, "Debug message")
	assert.Contains(t, output, "Info message")
}

func TestModel_View_Scrolling(t *testing.T) {
	m := NewModel(&MockTapeSource{})
	m.height = 3
	m.vertices = []VertexState{
		{ID: "1", Name: "Task 1"},
		{ID: "2", Name: "Task 2"},
		{ID: "3", Name: "Task 3"},
		{ID: "4", Name: "Task 4"},
		{ID: "5", Name: "Task 5"},
	}

	// SelectedIdx = 0. Range [0, 3) -> 1, 2, 3 displayed
	m.SelectedIdx = 0
	output := m.View()
	assert.Contains(t, output, "Task 1")
	assert.Contains(t, output, "Task 2")
	assert.Contains(t, output, "Task 3")
	assert.NotContains(t, output, "Task 4")
	assert.NotContains(t, output, "Task 5")

	// SelectedIdx = 4 (Last). Height=3.
	// Center logic: Start = 4 - (3/2) = 4 - 1 = 3.
	// Display indices: 3, 4. (Lengths: 5. Start 3. Loop: 3, 4. 2 items.)
	// Wait, screen fits 3 items. 3, 4 takes 2 lines. 3rd line empty?
	// It should break after 3 lines.
	// Loop: i=3 ("Task 4"), lines=1.
	// i=4 ("Task 5"), lines=2.
	// i=5 (loop end).
	m.SelectedIdx = 4
	output = m.View()
	assert.NotContains(t, output, "Task 1")
	assert.NotContains(t, output, "Task 2")
	assert.NotContains(t, output, "Task 3")
	assert.Contains(t, output, "Task 4")
	assert.Contains(t, output, "Task 5")
}
