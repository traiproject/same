package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Private brand colors.
	colorIris  = lipgloss.Color("#8B5CF6")
	colorSlate = lipgloss.Color("#667085")
	colorWhite = lipgloss.Color("#FFFFFF")
	colorInk   = lipgloss.Color("#0B0F19")
	colorMist  = lipgloss.Color("#F6F7FB")
	colorGreen = lipgloss.Color("#22A06B")
	colorRed   = lipgloss.Color("#D93025")
	_          = colorInk // Silence unused warning
	_          = colorMist

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
			Foreground(colorGreen)

	taskErrorStyle = lipgloss.NewStyle().
			Foreground(colorRed)

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

	// Tree connector styles.
	treeConnectorStyle = lipgloss.NewStyle().
				Foreground(colorSlate).
				Faint(true)

	expanderStyle = lipgloss.NewStyle().
			Foreground(colorIris)
)
