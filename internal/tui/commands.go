// Package tui provides a terminal user interface for visualizing build events.
package tui

import (
	"io"

	"github.com/charmbracelet/bubbletea"
	"github.com/vito/progrock"
)

// TapeSource is an interface for reading progrock updates.
// Since *progrock.Tape does not implement Read(), we define this interface
// for the caller to provide a valid source (e.g. an RPC client or a wrapper).
type TapeSource interface {
	Read() (*progrock.StatusUpdate, error)
}

// WaitForTape returns a Bubble Tea command that reads the next update from the tape.
// It returns MsgTapeUpdate on success or MsgTapeEnded on EOF or error.
func WaitForTape(tape TapeSource) tea.Cmd {
	return func() tea.Msg {
		update, err := tape.Read()
		if err != nil {
			if err == io.EOF {
				return MsgTapeEnded{}
			}
			// Treat other errors as end of stream for now
			return MsgTapeEnded{}
		}
		return MsgTapeUpdate{Update: update}
	}
}
