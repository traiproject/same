package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.trai.ch/same/internal/adapters/telemetry"
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
	Term   *Vterm
	Cached bool
}

// Model represents the main TUI state.
type Model struct {
	Tasks          []*TaskNode
	TaskMap        map[string]*TaskNode
	SpanMap        map[string]*TaskNode
	AutoScroll     bool
	ActiveTaskName string
	SelectedIdx    int
	ListOffset     int
	ListHeight     int
	LogWidth       int
	LogHeight      int
	FollowMode     bool
}

// Init initializes the model.
//
//nolint:gocritic // hugeParam ignored
func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) ensureVisible() {
	if m.ListHeight <= 0 {
		return
	}
	if m.SelectedIdx < m.ListOffset {
		m.ListOffset = m.SelectedIdx
	} else if m.SelectedIdx >= m.ListOffset+m.ListHeight {
		m.ListOffset = m.SelectedIdx - m.ListHeight + 1
	}
}

func (m *Model) getSelectedTask() *TaskNode {
	if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Tasks) {
		return m.Tasks[m.SelectedIdx]
	}
	return nil
}

func (m *Model) updateActiveView() {
	if node := m.getSelectedTask(); node != nil {
		m.ActiveTaskName = node.Name

		// Ensure term size is correct if we just switched
		if m.FollowMode && m.AutoScroll {
			// Calculate max offset: UsedHeight - Height
			maxOff := node.Term.UsedHeight() - node.Term.Height
			if maxOff < 0 {
				maxOff = 0
			}
			node.Term.Offset = maxOff
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
				m.ensureVisible()
				m.updateActiveView()
			}
		case "j", "down":
			if m.SelectedIdx < len(m.Tasks)-1 {
				m.SelectedIdx++
				m.FollowMode = false
				m.ensureVisible()
				m.updateActiveView()
			}
		case "esc":
			m.FollowMode = true
			// Jump to the currently running task if any.
			for i, t := range m.Tasks {
				if t.Status == StatusRunning {
					m.SelectedIdx = i
					break
				}
			}
			m.ensureVisible()
			m.updateActiveView()

		default:
			// Forward keys to the active task's terminal if applicable
			if m.ActiveTaskName != "" {
				if node, ok := m.TaskMap[m.ActiveTaskName]; ok {
					node.Term.Update(msg)
				}
			}
		}

	case tea.WindowSizeMsg:
		// Split screen: 30% for task list, 70% for logs
		listWidth := int(float64(msg.Width) * taskListWidthRatio)
		logWidth := msg.Width - listWidth - logPaneBorderWidth // minus margins/borders

		// Calculate header height dynamically
		headerHeight := lipgloss.Height(titleStyle.Render("TEST"))
		logHeight := msg.Height - headerHeight

		// Store calculated dimensions for future tasks
		m.LogWidth = logWidth
		m.LogHeight = logHeight

		// Calculate ListHeight with full header including newlines
		fullHeader := titleStyle.Render("TASKS") + "\n\n"
		listInfoHeight := lipgloss.Height(fullHeader)
		m.ListHeight = msg.Height - listInfoHeight
		m.ensureVisible()

		// Update all terminals
		for _, node := range m.Tasks {
			node.Term.SetWidth(logWidth)
			node.Term.SetHeight(logHeight)
		}

	case telemetry.MsgInitTasks:
		m.Tasks = make([]*TaskNode, len(msg.Tasks))
		m.TaskMap = make(map[string]*TaskNode, len(msg.Tasks))
		m.SpanMap = make(map[string]*TaskNode)
		for i, name := range msg.Tasks {
			term := NewVterm()
			// If we know the dimensions, set them immediately
			if m.LogWidth > 0 && m.LogHeight > 0 {
				term.SetWidth(m.LogWidth)
				term.SetHeight(m.LogHeight)
			}

			m.Tasks[i] = &TaskNode{
				Name:   name,
				Status: StatusPending,
				Term:   term,
			}
			m.TaskMap[name] = m.Tasks[i]
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
				m.ensureVisible()
				m.updateActiveView()
			}
		}

	case telemetry.MsgTaskLog:
		if node, ok := m.SpanMap[msg.SpanID]; ok {
			_, _ = node.Term.Write(msg.Data)
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
