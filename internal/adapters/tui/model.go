package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.trai.ch/bob/internal/adapters/telemetry"
)

const (
	taskListWidthRatio = 0.3
	logPaneBorderWidth = 4
)

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	// StatusPending indicates the task is waiting to start.
	StatusPending TaskStatus = "Pending"
	// StatusRunning indicates the task is currently executing.
	StatusRunning TaskStatus = "Running"
	// StatusDone indicates the task completed successfully.
	StatusDone TaskStatus = "Done"
	// StatusError indicates the task failed.
	StatusError TaskStatus = "Error"
)

// TaskNode represents a single task in the UI list.
type TaskNode struct {
	Name   string
	Status TaskStatus
	Logs   []byte
	Cached bool
}

// Model represents the main TUI state.
type Model struct {
	Tasks          []TaskNode
	TaskMap        map[string]*TaskNode
	SpanMap        map[string]*TaskNode
	Viewport       viewport.Model
	AutoScroll     bool
	ActiveTaskName string
	SelectedIdx    int
	FollowMode     bool
}

// Init initializes the model.
//
//nolint:gocritic // hugeParam ignored
func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) getSelectedTask() *TaskNode {
	if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Tasks) {
		return &m.Tasks[m.SelectedIdx]
	}
	return nil
}

func (m *Model) updateActiveView() {
	if node := m.getSelectedTask(); node != nil {
		m.ActiveTaskName = node.Name
		m.Viewport.SetContent(wrapLog(string(node.Logs), m.Viewport.Width))
		// If following, auto-scroll to bottom
		if m.FollowMode && m.AutoScroll {
			m.Viewport.GotoBottom()
		}
	}
}

// Update handles incoming messages and updates the model state.
//
//nolint:cyclop,gocritic // hugeParam ignored, cyclop ignored
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "k", "up":
			if m.SelectedIdx > 0 {
				m.SelectedIdx--
				m.FollowMode = false
				m.updateActiveView()
			}
		case "j", "down":
			if m.SelectedIdx < len(m.Tasks)-1 {
				m.SelectedIdx++
				m.FollowMode = false
				m.updateActiveView()
			}
		case "esc":
			m.FollowMode = true
			// When returning to follow mode, find the running task or last task
			// For now, let's just re-enable follow mode. The next MsgTaskStart
			// or manual navigation will handle selection.
			// Actually, better user experience: jump to the currently running task if any.
			for i, t := range m.Tasks {
				if t.Status == StatusRunning {
					m.SelectedIdx = i
					break
				}
			}
			m.updateActiveView()
		}

	case tea.WindowSizeMsg:
		// Split screen: 30% for task list, 70% for logs
		listWidth := int(float64(msg.Width) * taskListWidthRatio)
		logWidth := msg.Width - listWidth - logPaneBorderWidth // minus margins/borders

		m.Viewport.Width = logWidth
		m.Viewport.Height = msg.Height - 2 // minus header/footer space if any

		// Re-wrap content if we have an active task
		if m.ActiveTaskName != "" {
			if node, ok := m.TaskMap[m.ActiveTaskName]; ok {
				m.Viewport.SetContent(wrapLog(string(node.Logs), m.Viewport.Width))
			}
		}

	case telemetry.MsgInitTasks:
		m.Tasks = make([]TaskNode, len(msg.Tasks))
		m.TaskMap = make(map[string]*TaskNode, len(msg.Tasks))
		m.SpanMap = make(map[string]*TaskNode)
		for i, name := range msg.Tasks {
			m.Tasks[i] = TaskNode{
				Name:   name,
				Status: StatusPending,
			}
			m.TaskMap[name] = &m.Tasks[i]
		}

	case telemetry.MsgTaskStart:
		if node, ok := m.TaskMap[msg.Name]; ok {
			node.Status = StatusRunning
			m.SpanMap[msg.SpanID] = node

			// Focus follows activity ONLY if FollowMode is true
			if m.FollowMode {
				m.ActiveTaskName = msg.Name
				// Find index of this task
				for i, t := range m.Tasks {
					if t.Name == msg.Name {
						m.SelectedIdx = i
						break
					}
				}
				m.Viewport.SetContent(wrapLog(string(node.Logs), m.Viewport.Width))
				if m.AutoScroll {
					m.Viewport.GotoBottom()
				}
			}
		}

	case telemetry.MsgTaskLog:
		if node, ok := m.SpanMap[msg.SpanID]; ok {
			node.Logs = append(node.Logs, msg.Data...)

			// Truncate if too large (keep last 1MB just to be safe and efficient)
			const maxLogSize = 1024 * 1024
			if len(node.Logs) > maxLogSize {
				node.Logs = node.Logs[len(node.Logs)-maxLogSize:]
			}

			// Update viewport if we are looking at this task
			if node.Name == m.ActiveTaskName {
				// We append properly by setting content again.
				// Optimization: In a real app we might append line by line, but SetContent is safe.
				m.Viewport.SetContent(wrapLog(string(node.Logs), m.Viewport.Width))
				if m.AutoScroll {
					m.Viewport.GotoBottom()
				}
			}
		}

	case telemetry.MsgTaskComplete:
		if node, ok := m.SpanMap[msg.SpanID]; ok {
			if msg.Err != nil {
				node.Status = StatusError
			} else {
				node.Status = StatusDone
			}
		}
	}

	return m, cmd
}

func wrapLog(text string, width int) string {
	if width <= 0 {
		return text
	}
	if text == "" {
		return ""
	}

	return lipgloss.NewStyle().Width(width).Render(text)
}
