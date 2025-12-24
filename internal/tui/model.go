package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/vito/progrock"
)

// VertexState represents the current state of a task vertex in the TUI.
type VertexState struct {
	ID     string
	Name   string
	Status string // "running", "completed", "failed"
}

// Model is the Bubble Tea model for the TUI, managing vertices and tape updates.
type Model struct {
	tape     TapeSource
	vertices []VertexState
	width    int
	height   int
}

// NewModel creates a new TUI model with the given tape source.
func NewModel(tape TapeSource) Model {
	return Model{
		tape: tape,
	}
}

// Init initializes the model and starts reading from the tape.
func (m Model) Init() tea.Cmd {
	return WaitForTape(m.tape)
}

// Update handles incoming messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case MsgTapeUpdate:
		m.processVertexUpdates(msg.Update)
		return m, WaitForTape(m.tape)

	case MsgTapeEnded:
		return m, tea.Quit
	}

	return m, nil
}

// processVertexUpdates processes vertex updates from the tape.
func (m *Model) processVertexUpdates(update *progrock.StatusUpdate) {
	for _, v := range update.Vertexes {
		m.updateOrAddVertex(v)
	}
}

// updateOrAddVertex updates an existing vertex or adds a new one.
func (m *Model) updateOrAddVertex(v *progrock.Vertex) {
	for i, existing := range m.vertices {
		if existing.ID == v.Id {
			m.updateVertexStatus(i, v)
			return
		}
	}
	// Vertex not found, add it
	m.vertices = append(m.vertices, VertexState{
		ID:     v.Id,
		Name:   v.Name,
		Status: "running",
	})
}

// updateVertexStatus updates the status of an existing vertex.
func (m *Model) updateVertexStatus(index int, v *progrock.Vertex) {
	if v.Completed != nil {
		if v.Error != nil {
			m.vertices[index].Status = "failed"
		} else {
			m.vertices[index].Status = "completed"
		}
	}
}

// View renders the current state of the model as a string.
func (m Model) View() string {
	return "Waiting for events..."
}
