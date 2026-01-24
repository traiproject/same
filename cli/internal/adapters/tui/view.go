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

	s.WriteString(titleStyle.Render("BUILD PLAN") + "\n\n")

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

	// Calculate maximum name width for alignment
	maxNameWidth := m.calculateMaxNameWidth(start, end)

	for i := start; i < end; i++ {
		node := m.FlatList[i]
		s.WriteString(m.renderTreeRow(i, node, maxNameWidth) + "\n")
	}

	return s.String()
}

func (m *Model) renderTreeRow(index int, node *TaskNode, maxNameWidth int) string {
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

	// Status display (use canonical node for times)
	status := m.formatStatus(canonical)

	// Calculate padding for alignment
	nameWidth := calculateRowNameWidth(node)
	padding := strings.Repeat(" ", maxNameWidth-nameWidth)

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

	content := fmt.Sprintf("%s%s%s%s %s%s %s",
		indent, connector, expander, icon, node.Name, padding, status)

	return cursor + style.Render(content)
}

func isLastChild(node *TaskNode) bool {
	if node.Parent == nil {
		return false
	}
	children := node.Parent.Children
	return len(children) > 0 && children[len(children)-1] == node
}

func (m *Model) formatStatus(node *TaskNode) string {
	switch node.Status {
	case StatusPending:
		return "[Pending]"

	case StatusRunning:
		var duration time.Duration
		startTime := node.StartTime
		if !node.ExecStartTime.IsZero() {
			startTime = node.ExecStartTime
		}
		duration = time.Since(startTime)
		return fmt.Sprintf("[Running %s]", formatDuration(duration))

	case StatusDone:
		var duration time.Duration
		startTime := node.StartTime
		if !node.ExecStartTime.IsZero() {
			startTime = node.ExecStartTime
		}
		duration = node.EndTime.Sub(startTime)

		if node.Cached {
			return fmt.Sprintf("[Cached %s]", formatDuration(duration))
		}
		return fmt.Sprintf("[Took %s]", formatDuration(duration))

	case StatusError:
		var duration time.Duration
		startTime := node.StartTime
		if !node.ExecStartTime.IsZero() {
			startTime = node.ExecStartTime
		}
		duration = node.EndTime.Sub(startTime)
		return fmt.Sprintf("[Failed %s]", formatDuration(duration))

	default:
		return ""
	}
}

func formatDuration(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", duration.Seconds())
}

func calculateRowNameWidth(node *TaskNode) int {
	width := 0

	// Indent: 2 chars per depth level
	width += node.Depth * 2

	// Connector: 4 chars if depth > 0
	if node.Depth > 0 {
		width += 4
	}

	// Expander: 2 chars always
	width += 2

	// Icon: 1 char
	width++

	// Space separator before name
	width++

	// Name width (Unicode-safe)
	width += lipgloss.Width(node.Name)

	return width
}

func (m *Model) calculateMaxNameWidth(start, end int) int {
	maxWidth := 0

	for i := start; i < end; i++ {
		if i >= len(m.FlatList) {
			break
		}
		node := m.FlatList[i]
		width := calculateRowNameWidth(node)
		if width > maxWidth {
			maxWidth = width
		}
	}

	return maxWidth
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

	status := m.formatStatus(node)
	header = titleStyle.Render(fmt.Sprintf("LOGS: %s %s | Press ESC to return",
		node.Name, status))

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
