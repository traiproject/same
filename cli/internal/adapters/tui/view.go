package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI.
//
//nolint:gocritic // hugeParam ignored
func (m *Model) View() string {
	if m.ListHeight == 0 {
		return "Initializing..."
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.taskList(),
		m.logPane(),
	)
}

//nolint:gocritic // hugeParam ignored
func (m *Model) taskList() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("TASKS") + "\n\n")

	start := m.ListOffset
	end := m.ListOffset + m.ListHeight
	if end > len(m.Tasks) {
		end = len(m.Tasks)
	}
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		task := m.Tasks[i]
		s.WriteString(m.renderTaskRow(i, task) + "\n")
	}

	return listStyle.Render(s.String())
}

func (m *Model) renderTaskRow(index int, task *TaskNode) string {
	icon := m.getTaskIcon(task)
	style := m.getTaskStyle(task)

	// Highlight selected task
	var cursor string
	if index == m.SelectedIdx {
		cursor = selectedStyle.Render("> ")
		// If not Done/Error, highlight the text with Iris as well
		if task.Status != StatusDone && task.Status != StatusError {
			style = selectedStyle
		}
	} else {
		cursor = "  "
	}

	content := fmt.Sprintf("%s %s", icon, task.Name)
	return cursor + style.Render(content)
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

//nolint:gocritic // hugeParam ignored
func (m *Model) logPane() string {
	var header string
	var content string

	if m.ActiveTaskName != "" {
		status := ""
		if m.FollowMode {
			status = " (Following)"
		} else {
			status = " (Manual)"
		}
		header = titleStyle.Render("LOGS: " + m.ActiveTaskName + status)

		if node, ok := m.TaskMap[m.ActiveTaskName]; ok {
			content = node.Term.View()
		}
	} else {
		header = titleStyle.Render("LOGS (Waiting...)")
	}

	return logStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			content,
		),
	)
}
