// Package detector provides environment detection for output mode selection.
package detector

import (
	"os"

	"golang.org/x/term"
)

// OutputMode represents the rendering mode for the application.
type OutputMode int

const (
	// ModeAuto automatically detects the appropriate mode.
	ModeAuto OutputMode = iota
	// ModeTUI forces the interactive TUI renderer.
	ModeTUI
	// ModeLinear forces the linear CI renderer.
	ModeLinear
)

// DetectEnvironment returns the recommended output mode based on the environment.
// It checks if stdout is a TTY and if CI environment variables are set.
func DetectEnvironment() OutputMode {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	ci := os.Getenv("CI")
	isCI := ci == "true" || ci == "1"

	if !isTTY || isCI {
		return ModeLinear
	}
	return ModeTUI
}

// ResolveMode applies user override flag to auto-detection.
// userFlag should be one of: "auto", "tui", "linear", "ci", or empty.
func ResolveMode(autoDetected OutputMode, userFlag string) OutputMode {
	switch userFlag {
	case "tui":
		return ModeTUI
	case "linear", "ci":
		return ModeLinear
	case "auto", "":
		return autoDetected
	default:
		return autoDetected
	}
}
