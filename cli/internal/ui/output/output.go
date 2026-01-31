// Package output provides utilities for creating termenv.Output with consistent
// color profile and TTY handling across the CLI.
package output

import (
	"io"
	"os"

	"github.com/muesli/termenv"
)

// ColorProfile returns the color profile to use for interactive TUI environments.
// It checks if NO_COLOR is set, returning Ascii if so.
// Otherwise, it detects the terminal's capabilities automatically.
// For CI environments, consider using ColorProfileANSI() instead.
func ColorProfile() termenv.Profile {
	if os.Getenv("NO_COLOR") != "" {
		return termenv.Ascii
	}
	return termenv.EnvColorProfile()
}

// ColorProfileANSI returns the color profile for CI/non-interactive environments.
// It checks if NO_COLOR is set, returning Ascii if so.
// Otherwise, it returns ANSI for broad compatibility with CI systems.
func ColorProfileANSI() termenv.Profile {
	if os.Getenv("NO_COLOR") != "" {
		return termenv.Ascii
	}
	return termenv.ANSI
}

// New creates a new termenv.Output with the specific profile logic.
func New(w io.Writer, opts ...termenv.OutputOption) *termenv.Output {
	if w == nil {
		w = os.Stderr
	}

	opts = append(opts,
		termenv.WithProfile(ColorProfile()),
		termenv.WithTTY(true),
	)

	return termenv.NewOutput(w, opts...)
}

// NewWithProfile creates a new termenv.Output with a custom profile selector.
func NewWithProfile(w io.Writer, profileFn func() termenv.Profile, opts ...termenv.OutputOption) *termenv.Output {
	if w == nil {
		w = os.Stderr
	}

	opts = append(opts,
		termenv.WithProfile(profileFn()),
		termenv.WithTTY(true),
	)

	return termenv.NewOutput(w, opts...)
}
