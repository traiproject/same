package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Pane Styles.
	listStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			MarginRight(1).
			PaddingRight(1)

	logStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	// Task Status Styles.
	taskPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("246")) // Gray

	taskRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")). // Yellow/Gold
				Bold(true)

	taskDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")) // Green

	taskErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	taskCachedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")). // Dimmed Gray
			Faint(true)

	// Header Styles.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230"))
)
