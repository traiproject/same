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
	if m.Viewport.Height == 0 {
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
		var style lipgloss.Style
		var icon string

		// Determine style and icon based on status
		switch task.Status {
		case StatusRunning:
			style = taskRunningStyle
			icon = "●"
		case StatusDone:
			style = taskDoneStyle
			icon = "✓"
		case StatusError:
			style = taskErrorStyle
			icon = "✗"
		default: // Pending
			style = taskPendingStyle
			icon = "○"
		}

		// Override if cached
		if task.Cached {
			style = taskCachedStyle
			icon = "⚡"
		}

		// Highlight selected task
		line := fmt.Sprintf("%s %s", icon, task.Name)
		if i == m.SelectedIdx {
			// Selected row gets a pointer and distinct style/color if we had one.
			// For now, using the pointer prefix as requested.
			line = "> " + line
		} else {
			line = "  " + line
		}

		s.WriteString(style.Render(line) + "\n")
	}

	return listStyle.Render(s.String())
}

//nolint:gocritic // hugeParam ignored
func (m *Model) logPane() string {
	var header string
	if m.ActiveTaskName != "" {
		status := ""
		if m.FollowMode {
			status = " (Following)"
		} else {
			status = " (Manual)"
		}
		header = titleStyle.Render("LOGS: " + m.ActiveTaskName + status)
	} else {
		header = titleStyle.Render("LOGS (Waiting...)")
	}

	return logStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			m.Viewport.View(),
		),
	)
}
