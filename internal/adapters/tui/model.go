package tui

import (
	"bytes"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
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
	Name        string
	Status      TaskStatus
	Logs        []byte
	ViewContent string
	Cached      bool
}

// Model represents the main TUI state.
type Model struct {
	Tasks          []*TaskNode
	TaskMap        map[string]*TaskNode
	SpanMap        map[string]*TaskNode
	Viewport       viewport.Model
	AutoScroll     bool
	ActiveTaskName string
	SelectedIdx    int
	ListOffset     int
	ListHeight     int
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
		m.Viewport.SetContent(node.ViewContent)
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
			m.ensureVisible()
			m.updateActiveView()
		}

	case tea.WindowSizeMsg:
		// Split screen: 30% for task list, 70% for logs
		listWidth := int(float64(msg.Width) * taskListWidthRatio)
		logWidth := msg.Width - listWidth - logPaneBorderWidth // minus margins/borders

		m.Viewport.Width = logWidth

		// Calculate header height dynamically
		headerHeight := lipgloss.Height(titleStyle.Render("TEST"))
		m.Viewport.Height = msg.Height - headerHeight

		// Calculate ListHeight with full header including newlines
		fullHeader := titleStyle.Render("TASKS") + "\n\n"
		listInfoHeight := lipgloss.Height(fullHeader)
		m.ListHeight = msg.Height - listInfoHeight
		m.ensureVisible()

		// Re-wrap ALL content on window resize
		for _, node := range m.Tasks {
			node.ViewContent = wrapLog(string(node.Logs), m.Viewport.Width)
		}

		// Update viewport if we have an active task
		if m.ActiveTaskName != "" {
			if node, ok := m.TaskMap[m.ActiveTaskName]; ok {
				m.Viewport.SetContent(node.ViewContent)
			}
		}

	case telemetry.MsgInitTasks:
		m.Tasks = make([]*TaskNode, len(msg.Tasks))
		m.TaskMap = make(map[string]*TaskNode, len(msg.Tasks))
		m.SpanMap = make(map[string]*TaskNode)
		for i, name := range msg.Tasks {
			m.Tasks[i] = &TaskNode{
				Name:   name,
				Status: StatusPending,
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
				m.Viewport.SetContent(node.ViewContent)
				if m.AutoScroll {
					m.Viewport.GotoBottom()
				}
			}
		}

	case telemetry.MsgTaskLog:
		if node, ok := m.SpanMap[msg.SpanID]; ok {
			node.Logs = append(node.Logs, msg.Data...)

			truncated := false

			// Truncate if too large (keep last 1MB just to be safe and efficient)
			const maxLogSize = 1024 * 1024
			if len(node.Logs) > maxLogSize {
				targetStart := len(node.Logs) - maxLogSize
				cutIndex := targetStart

				// 1. Try to find a newline after the target start to keep log lines intact
				// We scan a bit forward from the cut point.
				// Slice from targetStart to end
				searchSlice := node.Logs[targetStart:]
				if idx := bytes.IndexByte(searchSlice, '\n'); idx != -1 {
					// Found a newline, cut right after it
					cutIndex = targetStart + idx + 1
				} else {
					// 2. If no newline found (extremely long line), ensure we don't split a UTF-8 rune.
					// Check if we are incorrectly starting in the middle of a rune.
					// utf8.RuneStart returns true if the byte is a start of a rune (or ASCII).
					// If node.Logs[cutIndex] is not a start byte, we advance until we find one.
					for cutIndex < len(node.Logs) && !utf8.RuneStart(node.Logs[cutIndex]) {
						cutIndex++
					}
				}

				// Apply the cut
				if cutIndex < len(node.Logs) {
					// Check if we are actually cutting something
					if cutIndex > 0 {
						node.Logs = node.Logs[cutIndex:]
						truncated = true
					}
				} else {
					// If we advanced past the end (should be rare/impossible unless trailing partial rune), clear logs
					node.Logs = nil
					truncated = true
				}
			}

			if truncated {
				// Slow Path: Truncation occurred, so we must regenerate the view content
				// to ensure it matches the new state of Logs.
				node.ViewContent = wrapLog(string(node.Logs), m.Viewport.Width)
			} else {
				// Fast Path: No truncation, just append the new wrapped content.
				// Incremental update: wrap only the new data and append
				newContent := wrapLog(string(msg.Data), m.Viewport.Width)
				node.ViewContent += newContent
			}

			// Update viewport if we are looking at this task
			if node.Name == m.ActiveTaskName {
				m.Viewport.SetContent(node.ViewContent)
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

	return wordwrap.String(text, width)
}
