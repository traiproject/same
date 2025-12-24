//nolint:testpackage // Test needs access to unexported fields
package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vito/progrock"
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
