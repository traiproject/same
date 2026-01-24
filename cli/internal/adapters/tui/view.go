package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI.
//
//nolint:gocritic // hugeParam ignored
func (m *Model) View() string {
	if m.ListHeight == 0 {
		return "Initializing..."
	}

	switch m.ViewMode {
	case ViewModeTree:
		return m.treeView()
	case ViewModeLogs:
		return m.fullScreenLogView()
	default:
		return m.treeView()
	}
}

//nolint:gocritic // hugeParam ignored
func (m *Model) treeView() string {
	var s strings.Builder

	// Header: change title and style based on build state
	header := "BUILD PLAN"
	headerStyle := titleStyle
	if m.BuildFailed {
		header = "BUILD FAILED - Press 'q' to exit"
		headerStyle = failureTitleStyle
	}
	s.WriteString(headerStyle.Render(header) + "\n\n")

	// Handle empty plan
	if len(m.FlatList) == 0 {
		s.WriteString("  No tasks planned\n")
		return s.String()
	}

	start := m.ListOffset
	end := m.ListOffset + m.ListHeight
	if end > len(m.FlatList) {
		end = len(m.FlatList)
	}
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		node := m.FlatList[i]
		s.WriteString(m.renderTreeRow(i, node) + "\n")
	}

	return s.String()
}

func (m *Model) renderTreeRow(index int, node *TaskNode) string {
	// Get live status from canonical node
	canonical := node.CanonicalNode
	if canonical == nil {
		canonical = node // Fallback if no canonical node set
	}

	icon := m.getTaskIcon(canonical)
	style := m.getTaskStyle(canonical)

	// Build tree connector based on depth
	indent := strings.Repeat("  ", node.Depth)
	var connector string
	if node.Depth > 0 {
		if isLastChild(node) {
			connector = "└── "
		} else {
			connector = "├── "
		}
	}

	// Expansion indicator
	var expander string
	if len(node.Children) > 0 {
		if node.IsExpanded {
			expander = "▼ "
		} else {
			expander = "▶ "
		}
	} else {
		expander = "  "
	}

	// Duration display (use canonical node for times)
	duration := m.formatDuration(canonical)

	// Selection cursor
	var cursor string
	if index == m.SelectedIdx {
		cursor = selectedStyle.Render("> ")
		if canonical.Status != StatusDone && canonical.Status != StatusError {
			style = selectedStyle
		}
	} else {
		cursor = "  "
	}

	content := fmt.Sprintf("%s%s%s%s %s %s",
		indent, connector, expander, icon, node.Name, duration)

	return cursor + style.Render(content)
}

func isLastChild(node *TaskNode) bool {
	if node.Parent == nil {
		return false
	}
	children := node.Parent.Children
	return len(children) > 0 && children[len(children)-1] == node
}

func (m *Model) formatDuration(node *TaskNode) string {
	if node.Status == StatusPending {
		return ""
	}

	var duration time.Duration
	startTime := node.StartTime
	if !node.ExecStartTime.IsZero() {
		startTime = node.ExecStartTime
	}

	if node.Status == StatusRunning {
		duration = time.Since(startTime)
	} else {
		duration = node.EndTime.Sub(startTime)
	}

	if duration < time.Second {
		return fmt.Sprintf("[%dms]", duration.Milliseconds())
	}
	return fmt.Sprintf("[%.1fs]", duration.Seconds())
}

//nolint:gocritic // hugeParam ignored
func (m *Model) fullScreenLogView() string {
	var header string
	var content string

	if m.ActiveTaskName == "" {
		return "No task selected"
	}

	node, ok := m.TaskMap[m.ActiveTaskName]
	if !ok {
		return "Task not found"
	}

	status := ""
	switch node.Status {
	case StatusRunning:
		status = " (Running)"
	case StatusDone:
		status = " (Completed)"
	case StatusError:
		status = " (Failed)"
	default:
		status = " (Pending)"
	}

	duration := m.formatDuration(node)
	header = titleStyle.Render(fmt.Sprintf("LOGS: %s%s %s | Press ESC to return",
		node.Name, status, duration))

	content = node.Term.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
	)
}

func (m *Model) getTaskIcon(task *TaskNode) string {
	if task.Cached {
		return "⚡"
	}

	switch task.Status {
	case StatusRunning:
		return "●"
	case StatusDone:
		return "✓"
	case StatusError:
		return "✗"
	default: // Pending
		return "○"
	}
}

func (m *Model) getTaskStyle(task *TaskNode) lipgloss.Style {
	if task.Cached {
		return taskCachedStyle
	}

	switch task.Status {
	case StatusRunning:
		return taskRunningStyle
	case StatusDone:
		return taskDoneStyle
	case StatusError:
		return taskErrorStyle
	default: // Pending
		return taskPendingStyle
	}
}
