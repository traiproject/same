package tui

import (
	"io"
	"os"

	"github.com/muesli/termenv"
)

// ColorProfile returns the color profile to use.
// It checks if NO_COLOR is set, returning Ascii if so.
// Otherwise, it returns ANSI, forcing color support.
func ColorProfile() termenv.Profile {
	if os.Getenv("NO_COLOR") != "" {
		return termenv.Ascii
	}
	return termenv.TrueColor
}

// NewOutput creates a new termenv.Output with the specific profile logic.
func NewOutput(w io.Writer, opts ...termenv.OutputOption) *termenv.Output {
	if w == nil {
		w = os.Stderr
	}

	opts = append(opts,
		termenv.WithProfile(ColorProfile()),
		termenv.WithTTY(true),
	)

	return termenv.NewOutput(w, opts...)
}
