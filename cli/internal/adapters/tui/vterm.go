package tui

import (
	"bytes"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vito/midterm"
)

// Vterm mimics a terminal state for TUI rendering.
type Vterm struct {
	vt      *midterm.Terminal
	Offset  int
	Height  int
	Width   int
	Prefix  string
	viewBuf *bytes.Buffer
	mu      sync.Mutex
}

// NewVterm creates a new Vterm instance.
func NewVterm() *Vterm {
	return &Vterm{
		vt:      midterm.NewAutoResizingTerminal(),
		viewBuf: new(bytes.Buffer),
	}
}

// Write implements io.Writer to write output to the virtual terminal.
func (v *Vterm) Write(p []byte) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Stick to bottom if we're already there or if height is zero (not yet rendered)
	stickToBottom := v.Offset >= v.maxOffset()

	n, err := v.vt.Write(p)

	if stickToBottom {
		v.Offset = v.maxOffset()
	}

	return n, err
}

// SetHeight updates the view height and adjusts scrolling.
func (v *Vterm) SetHeight(h int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if h < 1 {
		h = 1
	}

	stickToBottom := v.Offset >= v.maxOffset()

	v.Height = h

	if stickToBottom {
		v.Offset = v.maxOffset()
	} else {
		// Clamp offset if the new height makes the current offset invalid
		limit := v.maxOffset()
		if v.Offset > limit {
			v.Offset = limit
		}
	}
}

// SetWidth updates the terminal width.
func (v *Vterm) SetWidth(w int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if w < 1 {
		w = 1
	}

	v.Width = w
	// Safe subtract
	cols := w - len(v.Prefix)
	if cols < 1 {
		cols = 1
	}
	v.vt.ResizeX(cols)
}

// UsedHeight returns the total number of lines in the terminal buffer.
func (v *Vterm) UsedHeight() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.vt.UsedHeight()
}

// View handles standard Bubble Tea view rendering.
func (v *Vterm) View() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return string(v.viewBytes())
}

func (v *Vterm) viewBytes() []byte {
	v.viewBuf.Reset()

	// Ensure offset is valid before rendering
	if v.Offset < 0 {
		v.Offset = 0
	}
	limit := v.maxOffset()
	if v.Offset > limit {
		v.Offset = limit
	}

	for i := 0; i < v.Height; i++ {
		row := v.Offset + i
		if row >= v.vt.UsedHeight() {
			break
		}

		if i > 0 {
			_ = v.viewBuf.WriteByte('\n')
		}

		_, _ = v.viewBuf.WriteString(v.Prefix)
		_ = v.vt.RenderLine(v.viewBuf, row)
	}

	// We copy the bytes because viewBuf is reused
	out := make([]byte, v.viewBuf.Len())
	copy(out, v.viewBuf.Bytes())
	return out
}

// Update handles incoming events, specifically for scrolling.
func (v *Vterm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			v.Offset--
		case "down", "j":
			v.Offset++
		case "pgup":
			v.Offset -= v.Height
		case "pgdown":
			v.Offset += v.Height
		case "home":
			v.Offset = 0
		case "end":
			v.Offset = v.maxOffset()
		}
	}

	// Clamp after adjustment
	if v.Offset < 0 {
		v.Offset = 0
	}
	limit := v.maxOffset()
	if v.Offset > limit {
		v.Offset = limit
	}

	return nil, nil
}

func (v *Vterm) maxOffset() int {
	maxOff := v.vt.UsedHeight() - v.Height
	if maxOff < 0 {
		return 0
	}
	return maxOff
}
