package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Viewport.Height == 0 {
		return "Initializing..."
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.taskList(),
		m.logPane(),
	)
}

func (m Model) taskList() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("TASKS") + "\n\n")

	for _, task := range m.Tasks {
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

		// Highlight active task with a marker or background (optional, strictly following simple requirements first)
		// For now, just render the name
		line := fmt.Sprintf("%s %s", icon, task.Name)
		if task.Name == m.ActiveTaskName {
			// Maybe add a pointer?
			line = "> " + line
		} else {
			line = "  " + line
		}

		s.WriteString(style.Render(line) + "\n")
	}

	return listStyle.Render(s.String())
}

func (m Model) logPane() string {
	var header string
	if m.ActiveTaskName != "" {
		header = titleStyle.Render("LOGS: " + m.ActiveTaskName)
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
