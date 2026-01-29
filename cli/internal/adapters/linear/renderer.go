// Package linear provides a synchronous, line-buffered renderer for CI environments.
package linear

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"sync"
	"time"

	"github.com/muesli/termenv"
	"go.trai.ch/same/internal/ui/output"
	"go.trai.ch/same/internal/ui/style"
)

var colorPalette = []termenv.Color{
	termenv.ANSICyan,
	termenv.ANSIMagenta,
	termenv.ANSIYellow,
	termenv.ANSIBlue,
	termenv.ANSIBrightCyan,
	termenv.ANSIBrightMagenta,
	termenv.ANSIBrightYellow,
	termenv.ANSIBrightBlue,
}

// Renderer implements ports.Renderer for CI/non-interactive environments.
// It outputs linear, chronological logs with task name prefixes.
type Renderer struct {
	stdout io.Writer
	stderr io.Writer
	output *termenv.Output

	mu      sync.Mutex
	tasks   map[string]*taskState // spanID -> task state
	buffers map[string]*bytes.Buffer
}

type taskState struct {
	name      string
	startTime time.Time
	color     termenv.Color
}

// NewRenderer creates a new LinearRenderer.
func NewRenderer(stdout, stderr io.Writer) *Renderer {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	// Create termenv.Output using shared output factory with ANSI profile for CI compatibility
	out := output.NewWithProfile(stderr, output.ColorProfileANSI)

	return &Renderer{
		stdout:  stdout,
		stderr:  stderr,
		output:  out,
		tasks:   make(map[string]*taskState),
		buffers: make(map[string]*bytes.Buffer),
	}
}

func assignColor(taskName string) termenv.Color {
	h := fnv.New32a()
	h.Write([]byte(taskName))
	hash := h.Sum32()
	idx := hash % uint32(len(colorPalette)) //nolint:gosec // palette size is small and constant
	return colorPalette[idx]
}

// Start is a no-op for linear renderer (synchronous).
func (r *Renderer) Start(_ context.Context) error {
	return nil
}

// Stop flushes all remaining buffers.
func (r *Renderer) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Flush all remaining buffers
	for spanID := range r.buffers {
		r.flushBufferLocked(spanID)
	}

	return nil
}

// Wait is a no-op for linear renderer (synchronous).
func (r *Renderer) Wait() error {
	return nil
}

// OnPlanEmit prints the planned tasks.
func (r *Renderer) OnPlanEmit(tasks []string, _ map[string][]string, targets []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, _ = fmt.Fprintf(r.stderr, "Planning to build %d task(s) for target(s): %v\n",
		len(tasks), targets)
}

// OnTaskStart prints a task start message.
func (r *Renderer) OnTaskStart(spanID, _ /* parentID */, name string, startTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	color := assignColor(name)
	r.tasks[spanID] = &taskState{
		name:      name,
		startTime: startTime,
		color:     color,
	}
	r.buffers[spanID] = new(bytes.Buffer)

	// Print start message to stderr
	prefix := r.output.String(fmt.Sprintf("[%s]", name)).Foreground(color).String()
	_, _ = fmt.Fprintf(r.stderr, "%s Starting...\n", prefix)
}

// OnTaskLog buffers log data and prints complete lines with task prefix.
func (r *Renderer) OnTaskLog(spanID string, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[spanID]
	if !ok {
		return
	}

	buf := r.buffers[spanID]
	buf.Write(data)

	// Process complete lines
	for {
		line, err := buf.ReadBytes('\n')
		if err != nil {
			// Incomplete line, put it back
			if len(line) > 0 {
				// Create a new buffer with the partial line
				newBuf := new(bytes.Buffer)
				newBuf.Write(line)
				r.buffers[spanID] = newBuf
			}
			break
		}

		// Print complete line with prefix
		r.printLineLocked(task.name, task.color, line)
	}
}

// OnTaskComplete flushes remaining buffer and prints completion status.
func (r *Renderer) OnTaskComplete(spanID string, endTime time.Time, err error, cached bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[spanID]
	if !ok {
		return
	}

	// Flush any remaining buffer
	r.flushBufferLocked(spanID)

	// Print completion message
	duration := endTime.Sub(task.startTime)
	durationStr := formatDuration(duration)
	coloredPrefix := r.output.String(fmt.Sprintf("[%s]", task.name)).Foreground(task.color).String()

	switch {
	case err != nil:
		symbol := r.output.String(style.Cross).Foreground(termenv.ANSIRed).String()
		_, _ = fmt.Fprintf(r.stderr, "%s %s Failed after %s: %v\n",
			coloredPrefix, symbol, durationStr, err)
	case cached:
		symbol := r.output.String(style.Tilde).Foreground(termenv.ANSIYellow).String()
		_, _ = fmt.Fprintf(r.stderr, "%s %s Cached (skipped in %s)\n",
			coloredPrefix, symbol, durationStr)
	default:
		symbol := r.output.String(style.Check).Foreground(termenv.ANSIGreen).String()
		_, _ = fmt.Fprintf(r.stderr, "%s %s Completed in %s\n",
			coloredPrefix, symbol, durationStr)
	}

	// Cleanup
	delete(r.tasks, spanID)
	delete(r.buffers, spanID)
}

// flushBufferLocked flushes any remaining data in the buffer for a task.
// Must be called with r.mu held.
func (r *Renderer) flushBufferLocked(spanID string) {
	task, ok := r.tasks[spanID]
	if !ok {
		return
	}

	buf := r.buffers[spanID]
	if buf.Len() > 0 {
		// Print the remaining partial line
		r.printLineLocked(task.name, task.color, buf.Bytes())
		buf.Reset()
	}
}

// printLineLocked prints a line with the task name prefix.
// Must be called with r.mu held.
func (r *Renderer) printLineLocked(taskName string, color termenv.Color, line []byte) {
	// Trim trailing newline for cleaner output
	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))

	if len(line) == 0 {
		return
	}

	prefix := r.output.String(fmt.Sprintf("[%s]", taskName)).Foreground(color).String()
	_, _ = fmt.Fprintf(r.stdout, "%s %s\n", prefix, string(line))
}

// formatDuration formats a duration with appropriate units.
// Uses no decimal places for µs/ms and one decimal place for seconds.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%.0fµs", float64(d)/float64(time.Microsecond))
	case d < time.Second:
		return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
}
