package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vito/progrock"
)

const (
	statusRunning   = "running"
	statusCompleted = "completed"
	statusFailed    = "failed"
	statusPending   = "pending"
)

// VertexState represents the current state of a task vertex in the TUI.
type VertexState struct {
	ID               string
	ParentID         string
	Name             string
	Status           string // statusRunning, statusCompleted, statusFailed, statusPending
	IndentationLevel int
}

type styles struct {
	running   lipgloss.Style
	completed lipgloss.Style
	failed    lipgloss.Style
	pending   lipgloss.Style
}

// Model is the Bubble Tea model for the TUI, managing vertices and tape updates.
type Model struct {
	tape     TapeSource
	vertices []VertexState
	width    int
	height   int
	spinner  spinner.Model
	styles   styles
}

// NewModel creates a new TUI model with the given tape source.
func NewModel(tape TapeSource) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("yellow"))

	return &Model{
		tape:    tape,
		spinner: s,
		styles: styles{
			running:   lipgloss.NewStyle().Foreground(lipgloss.Color("yellow")),
			completed: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),  // Green
			failed:    lipgloss.NewStyle().Foreground(lipgloss.Color("160")), // Red
			pending:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")), // Gray
		},
	}
}

// Init initializes the model and starts reading from the tape.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		WaitForTape(m.tape),
		m.spinner.Tick,
	)
}

// Update handles incoming messages and updates the model state.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg(msg)
	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)
	case MsgTapeUpdate:
		return m.handleTapeUpdate(msg)
	case MsgTapeEnded:
		return m, tea.Quit
	}
	return m, nil
}

// handleKeyMsg handles keyboard input messages.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	return m, nil
}

// handleWindowSizeMsg handles window resize messages.
func (m *Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	return m, nil
}

// handleSpinnerTick handles spinner animation tick messages.
func (m *Model) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// handleTapeUpdate handles tape update messages.
func (m *Model) handleTapeUpdate(msg MsgTapeUpdate) (tea.Model, tea.Cmd) {
	m.processVertexUpdates(msg.Update)
	return m, WaitForTape(m.tape)
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
		Status: statusRunning,
	})
}

// updateVertexStatus updates the status of an existing vertex.
func (m *Model) updateVertexStatus(index int, v *progrock.Vertex) {
	if v.Completed != nil {
		if v.Error != nil {
			m.vertices[index].Status = statusFailed
		} else {
			m.vertices[index].Status = statusCompleted
		}
	}
}

// View renders the current state of the model as a string.
func (m *Model) View() string {
	var s strings.Builder

	// Determine start index to handle overflow
	start := 0
	if len(m.vertices) > m.height && m.height > 0 {
		start = len(m.vertices) - m.height
	}

	for i := start; i < len(m.vertices); i++ {
		v := m.vertices[i]
		// Icon
		var icon string
		var style lipgloss.Style
		switch v.Status {
		case statusRunning:
			icon = m.spinner.View()
			style = m.styles.running
		case statusCompleted:
			icon = "✓"
			style = m.styles.completed
		case statusFailed:
			icon = "✗"
			style = m.styles.failed
		default:
			icon = "•"
			style = m.styles.pending
		}

		// Indentation
		indent := strings.Repeat("  ", v.IndentationLevel)

		// Line: [Indent][Icon] [Name]
		line := fmt.Sprintf("%s%s %s\n", indent, style.Render(icon), v.Name)
		s.WriteString(line)
	}

	return s.String()
}
