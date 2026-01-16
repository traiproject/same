package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Private brand colors.
	colorIris  = lipgloss.Color("#5D3FD3")
	colorSlate = lipgloss.Color("#667085")
	colorWhite = lipgloss.Color("#FFFFFF")
	colorInk   = lipgloss.Color("#0B0F19")
	_          = colorInk // Silence unused warning

	// Pane Styles.
	listStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(colorSlate).
			MarginRight(1).
			PaddingRight(1)

	logStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	// Task Status Styles.
	taskPendingStyle = lipgloss.NewStyle().
				Foreground(colorSlate)

	taskRunningStyle = lipgloss.NewStyle().
				Foreground(colorIris).
				Bold(true)

	taskDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")) // Green

	taskErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	taskCachedStyle = lipgloss.NewStyle().
			Foreground(colorSlate).
			Faint(true)

	// Selection Style.
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorIris).
			Bold(true)

	// Header Styles.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			Background(colorIris).
			Foreground(colorWhite)
)
