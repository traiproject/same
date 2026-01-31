// Package style provides shared UI styling primitives including brand colors
// and icons for consistent visual presentation across the CLI.
package style

import "github.com/charmbracelet/lipgloss"

// Brand Colors.
var (
	Iris   = lipgloss.Color("#8B5CF6")
	Slate  = lipgloss.Color("#667085")
	White  = lipgloss.Color("#FFFFFF")
	Ink    = lipgloss.Color("#0B0F19")
	Mist   = lipgloss.Color("#F6F7FB")
	Green  = lipgloss.Color("#22A06B")
	Red    = lipgloss.Color("#D93025")
	Yellow = lipgloss.Color("#F59E0B")
)

// Icons.
const (
	Check   = "✓"
	Cross   = "✗"
	Warning = "!"
	Tilde   = "~"
	Dot     = "●"
	Circle  = "○"
)
