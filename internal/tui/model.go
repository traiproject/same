package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vito/progrock"
	"go.trai.ch/bob/internal/core/domain"
)

const (
	statusRunning   = "running"
	statusCompleted = "completed"
	statusFailed    = "failed"
	statusPending   = "pending"

	maxLogLines = 5
)

// VertexState represents the current state of a task vertex in the TUI.
type VertexState struct {
	ID               string
	ParentID         string
	Name             string
	Status           string // statusRunning, statusCompleted, statusFailed, statusPending
	IndentationLevel int
	Expanded         bool
}

type styles struct {
	running   lipgloss.Style
	completed lipgloss.Style
	failed    lipgloss.Style
	pending   lipgloss.Style
	log       lipgloss.Style
}

// Model is the Bubble Tea model for the TUI, managing vertices and tape updates.
type Model struct {
	tape     TapeSource
	vertices []VertexState
	logs     map[string][]string
	width    int
	height   int
	spinner  spinner.Model
	styles   styles

	// Interactivity
	SelectedIdx int
	MinLogLevel domain.LogLevel
}

// NewModel creates a new TUI model with the given tape source.
func NewModel(tape TapeSource) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("yellow"))

	return &Model{
		tape:    tape,
		logs:    make(map[string][]string),
		spinner: s,
		styles: styles{
			running:   lipgloss.NewStyle().Foreground(lipgloss.Color("yellow")),
			completed: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),  // Green
			failed:    lipgloss.NewStyle().Foreground(lipgloss.Color("160")), // Red
			pending:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")), // Gray
			log:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")), // Light Gray
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
	case MsgVertexStarted:
		return m.handleVertexStarted(msg)
	case MsgVertexCompleted:
		return m.handleVertexCompleted(msg)
	case MsgLogReceived:
		return m.handleLogReceived(msg)
	case MsgTapeEnded:
		return m, tea.Quit
	}
	return m, nil
}

// handleKeyMsg handles keyboard input messages.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "up", "k", "down", "j":
		m.handleNavigation(msg.String())
	case "enter", " ":
		m.handleToggle()
	case "+", "-":
		m.handleVerbosity(msg.String())
	}
	return m, nil
}

func (m *Model) handleNavigation(key string) {
	if key == "up" || key == "k" {
		if m.SelectedIdx > 0 {
			m.SelectedIdx--
		} else {
			m.SelectedIdx = len(m.vertices) - 1
		}
	} else { // down or j
		if m.SelectedIdx < len(m.vertices)-1 {
			m.SelectedIdx++
		} else {
			m.SelectedIdx = 0
		}
	}
}

func (m *Model) handleToggle() {
	if len(m.vertices) > 0 {
		m.vertices[m.SelectedIdx].Expanded = !m.vertices[m.SelectedIdx].Expanded
	}
}

func (m *Model) handleVerbosity(key string) {
	if key == "+" {
		// Decrease min level (show more verbose/debug logs)
		if m.MinLogLevel > domain.LogLevelDebug {
			m.MinLogLevel -= 4
		}
	} else { // "-"
		// Increase min level (show only errors/warns)
		if m.MinLogLevel < domain.LogLevelError {
			m.MinLogLevel += 4
		}
	}
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
	cmds := make([]tea.Cmd, 0, len(msg.Update.Vertexes)+len(msg.Update.Logs)+1)

	// Process vertex updates
	for _, v := range msg.Update.Vertexes {
		if cmd := m.updateOrAddVertex(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Process logs
	for _, l := range msg.Update.Logs {
		cmds = append(cmds, func() tea.Msg {
			return MsgLogReceived{
				VertexID: l.Vertex,
				Stream:   l.Stream,
				Text:     string(l.Data),
			}
		})
	}

	cmds = append(cmds, WaitForTape(m.tape))
	return m, tea.Batch(cmds...)
}

// handleLogReceived stores received logs.
func (m *Model) handleLogReceived(msg MsgLogReceived) (tea.Model, tea.Cmd) {
	m.logs[msg.VertexID] = append(m.logs[msg.VertexID], msg.Text)

	// Auto-expand on "Task started." log
	if strings.Contains(msg.Text, "Task started.") {
		for i := range m.vertices {
			if m.vertices[i].ID == msg.VertexID {
				m.vertices[i].Expanded = true
			} else {
				m.vertices[i].Expanded = false
			}
		}
	}

	return m, nil
}

func (m *Model) handleVertexStarted(msg MsgVertexStarted) (tea.Model, tea.Cmd) {
	// Auto-Focus Strategy: Expand new, collapse others
	for i := range m.vertices {
		if m.vertices[i].ID == msg.ID {
			m.vertices[i].Expanded = true
		} else {
			m.vertices[i].Expanded = false
		}
	}
	return m, nil
}

func (m *Model) handleVertexCompleted(msg MsgVertexCompleted) (tea.Model, tea.Cmd) {
	// Auto-Collapse Strategy: Collapse on success, keep expanded on failure
	for i := range m.vertices {
		if m.vertices[i].ID == msg.ID {
			if msg.Err == nil {
				// Success
				m.vertices[i].Expanded = false
			} else {
				// Failure
				m.vertices[i].Expanded = true
			}
			break
		}
	}
	return m, nil
}

// updateOrAddVertex updates an existing vertex or adds a new one.
// Returns a command if an event occurred (Started, Completed).
func (m *Model) updateOrAddVertex(v *progrock.Vertex) tea.Cmd {
	for i, existing := range m.vertices {
		if existing.ID == v.Id {
			return m.updateVertexStatus(i, v)
		}
	}
	// Vertex not found, add it
	m.vertices = append(m.vertices, VertexState{
		ID:     v.Id,
		Name:   v.Name,
		Status: statusRunning,
	})

	return func() tea.Msg {
		return MsgVertexStarted{
			ID:   v.Id,
			Name: v.Name,
		}
	}
}

// updateVertexStatus updates the status of an existing vertex.
// Returns a command if the vertex completed.
func (m *Model) updateVertexStatus(index int, v *progrock.Vertex) tea.Cmd {
	vState := &m.vertices[index]

	// Check if already completed to avoid duplicate events
	if vState.Status == statusCompleted || vState.Status == statusFailed {
		return nil
	}

	if v.Completed != nil {
		var err error
		if v.Error != nil {
			vState.Status = statusFailed
			err = errors.New(*v.Error)
		} else {
			vState.Status = statusCompleted
		}

		return func() tea.Msg {
			return MsgVertexCompleted{
				ID:  v.Id,
				Err: err,
			}
		}
	}
	return nil
}

// View renders the current state of the model as a string.
func (m *Model) View() string {
	var s strings.Builder

	// Determine start index to keep SelectedIdx in view
	// Center the selected index in the window roughly
	start := 0
	if m.SelectedIdx > m.height/2 {
		start = m.SelectedIdx - (m.height / 2)
	}

	linesRendered := 0
	for i := start; i < len(m.vertices); i++ {
		// Stop if we've filled the screen
		if m.height > 0 && linesRendered >= m.height {
			break
		}

		line, logs := m.renderVertex(i, &m.vertices[i])
		s.WriteString(line)
		linesRendered++

		if logs != "" {
			s.WriteString(logs)
			linesRendered += strings.Count(logs, "\n")
		}
	}

	return s.String()
}

func (m *Model) renderVertex(i int, v *VertexState) (line, logs string) {
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

	// Cursor
	cursor := "  "
	if i == m.SelectedIdx {
		cursor = "> "
	}

	// Indentation
	indent := strings.Repeat("  ", v.IndentationLevel)

	// Line: [Cursor][Indent][Icon] [Name]
	line = fmt.Sprintf("%s%s%s %s\n", cursor, indent, style.Render(icon), v.Name)

	// Render logs if expanded
	if v.Expanded {
		logs = m.renderLogs(v.ID, cursor+indent)
	}

	return line, logs
}

func (m *Model) renderLogs(vertexID, indent string) string {
	logs, ok := m.logs[vertexID]
	if !ok || len(logs) == 0 {
		return ""
	}

	// Filter logs by level and collect the tail
	var filteredLogs []string
	for _, l := range logs {
		level := parseLogLevel(l)
		if level >= m.MinLogLevel {
			filteredLogs = append(filteredLogs, l)
		}
	}

	if len(filteredLogs) == 0 {
		return ""
	}

	// Show last N lines of FILTERED logs
	tailLines := filteredLogs
	if len(filteredLogs) > maxLogLines {
		tailLines = filteredLogs[len(filteredLogs)-maxLogLines:]
	}

	logBlock := strings.Join(tailLines, "\n")
	logStyle := m.styles.log.
		PaddingLeft(2).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderLeft(true)

	// Indent the log block to match vertex
	var sb strings.Builder
	for _, l := range strings.Split(logStyle.Render(logBlock), "\n") {
		sb.WriteString(indent + "  " + l + "\n")
	}
	return sb.String()
}

func parseLogLevel(line string) domain.LogLevel {
	switch {
	case strings.Contains(line, "[DEBUG]"):
		return domain.LogLevelDebug
	case strings.Contains(line, "[INFO]"):
		return domain.LogLevelInfo
	case strings.Contains(line, "[WARN]"):
		return domain.LogLevelWarn
	case strings.Contains(line, "[ERROR]"):
		return domain.LogLevelError
	}
	return domain.LogLevelInfo
}
