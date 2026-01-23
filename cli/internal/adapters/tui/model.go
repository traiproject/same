package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"go.trai.ch/same/internal/adapters/telemetry"
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

	// Tree structure
	Children      []*TaskNode
	Parent        *TaskNode
	IsExpanded    bool
	Depth         int
	CanonicalNode *TaskNode // Reference to the node in TaskMap for live status

	// Duration tracking
	StartTime time.Time
	EndTime   time.Time
}

// ViewMode represents the current view state of the TUI.
type ViewMode string

const (
	// ViewModeTree displays the task tree with dependencies.
	ViewModeTree ViewMode = "tree"
	// ViewModeLogs displays full-screen logs for a specific task.
	ViewModeLogs ViewMode = "logs"
)

// MsgTick is sent periodically to update running task durations.
type MsgTick time.Time

// Model represents the main TUI state.
type Model struct {
	Tasks          []*TaskNode
	TaskMap        map[string]*TaskNode
	SpanMap        map[string]*TaskNode
	Output         *termenv.Output
	AutoScroll     bool
	ActiveTaskName string
	SelectedIdx    int
	ListOffset     int
	ListHeight     int
	LogWidth       int
	LogHeight      int
	FollowMode     bool

	// Tree view
	TreeRoots    []*TaskNode
	FlatList     []*TaskNode
	ViewMode     ViewMode
	TickInterval time.Duration
	DisableTick  bool // Disable tick loop for testing
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
	if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.FlatList) {
		return m.FlatList[m.SelectedIdx]
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

		case "esc":
			if m.ViewMode == ViewModeLogs {
				// Return to tree view
				m.ViewMode = ViewModeTree
				m.FollowMode = true
				m.ensureVisible()
				if !m.DisableTick {
					return m, tea.Tick(m.TickInterval, func(t time.Time) tea.Msg {
						return MsgTick(t)
					})
				}
			}
			// Existing esc logic for follow mode
			m.FollowMode = true
			for i, node := range m.FlatList {
				canonical := node.CanonicalNode
				if canonical == nil {
					canonical = node
				}
				if canonical.Status == StatusRunning {
					m.SelectedIdx = i
					break
				}
			}
			m.ensureVisible()
			m.updateActiveView()

		case "enter":
			if m.ViewMode == ViewModeTree {
				// Switch to full-screen log view for selected task
				m.ViewMode = ViewModeLogs
				if node := m.getSelectedTask(); node != nil {
					m.ActiveTaskName = node.Name
				}
			}

		case " ": // Space
			if m.ViewMode == ViewModeTree {
				// Toggle expansion of selected node
				if node := m.getSelectedTask(); node != nil && len(node.Children) > 0 {
					node.IsExpanded = !node.IsExpanded
					// Rebuild flat list
					m.FlatList = flattenTree(m.TreeRoots)
					m.ensureVisible()
				}
			}

		case "k", "up":
			if m.ViewMode == ViewModeTree {
				if m.SelectedIdx > 0 {
					m.SelectedIdx--
					m.FollowMode = false
					m.ensureVisible()
				}
			} else {
				// Forward to terminal for log scrolling
				if node := m.getSelectedTask(); node != nil {
					node.Term.Update(msg)
				}
			}

		case "j", "down":
			if m.ViewMode == ViewModeTree {
				if m.SelectedIdx < len(m.FlatList)-1 {
					m.SelectedIdx++
					m.FollowMode = false
					m.ensureVisible()
				}
			} else {
				// Forward to terminal
				if node := m.getSelectedTask(); node != nil {
					node.Term.Update(msg)
				}
			}

		default:
			// Forward other keys to terminal when in log view
			if m.ViewMode == ViewModeLogs {
				if node := m.getSelectedTask(); node != nil {
					node.Term.Update(msg)
				}
			}
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(titleStyle.Render("BUILD PLAN"))

		if m.ViewMode == ViewModeTree {
			// Tree view: full width list
			m.LogWidth = msg.Width
			m.LogHeight = msg.Height - headerHeight - 2 // Reserve space for header

			fullHeader := titleStyle.Render("BUILD PLAN") + "\n\n"
			listInfoHeight := lipgloss.Height(fullHeader)
			m.ListHeight = msg.Height - listInfoHeight
			m.ensureVisible()
		} else {
			// Logs view: full screen terminal
			m.LogWidth = msg.Width
			m.LogHeight = msg.Height - headerHeight
		}

		// Update all terminals with new dimensions
		for _, node := range m.TaskMap {
			node.Term.SetWidth(m.LogWidth)
			node.Term.SetHeight(m.LogHeight)
		}

	case telemetry.MsgInitTasks:
		// Initialize TaskMap with all tasks
		m.TaskMap = make(map[string]*TaskNode, len(msg.Tasks))
		m.SpanMap = make(map[string]*TaskNode)

		for _, name := range msg.Tasks {
			term := NewVterm()
			if m.LogWidth > 0 && m.LogHeight > 0 {
				term.SetWidth(m.LogWidth)
				term.SetHeight(m.LogHeight)
			}

			m.TaskMap[name] = &TaskNode{
				Name:   name,
				Status: StatusPending,
				Term:   term,
			}
		}

		// Build tree structure
		m.TreeRoots = buildTree(msg.Targets, msg.Dependencies, m.TaskMap)
		m.FlatList = flattenTree(m.TreeRoots)

		// Initialize view mode
		m.ViewMode = ViewModeTree
		m.TickInterval = defaultTickInterval * time.Millisecond

		if !m.DisableTick {
			return m, tea.Tick(m.TickInterval, func(t time.Time) tea.Msg {
				return MsgTick(t)
			})
		}
		return m, nil

	case telemetry.MsgTaskStart:
		if node, ok := m.TaskMap[msg.Name]; ok {
			node.Status = StatusRunning
			node.StartTime = msg.StartTime
			m.SpanMap[msg.SpanID] = node

			// Focus follows activity ONLY if FollowMode is true
			if m.FollowMode {
				m.ActiveTaskName = msg.Name
				// Find index of this task in FlatList
				for i, t := range m.FlatList {
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
			node.EndTime = msg.EndTime
			if msg.Err != nil {
				node.Status = StatusError
			} else {
				node.Status = StatusDone
			}
		}

	case MsgTick:
		if m.ViewMode == ViewModeTree && !m.DisableTick {
			return m, tea.Tick(m.TickInterval, func(t time.Time) tea.Msg {
				return MsgTick(t)
			})
		}
	}

	return m, cmd
}
