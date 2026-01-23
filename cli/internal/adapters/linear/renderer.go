// Package linear provides a synchronous, line-buffered renderer for CI environments.
package linear

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/muesli/termenv"
)

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
}

// NewRenderer creates a new LinearRenderer.
func NewRenderer(stdout, stderr io.Writer) *Renderer {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	// Create termenv.Output for color support
	// WithColorCache(false) ensures TTY detection happens correctly
	profile := colorProfile()
	output := termenv.NewOutput(stderr, termenv.WithProfile(profile))

	return &Renderer{
		stdout:  stdout,
		stderr:  stderr,
		output:  output,
		tasks:   make(map[string]*taskState),
		buffers: make(map[string]*bytes.Buffer),
	}
}

// colorProfile returns the color profile based on environment.
func colorProfile() termenv.Profile {
	if os.Getenv("NO_COLOR") != "" {
		return termenv.Ascii
	}
	// Use ANSI for basic color support in CI
	return termenv.ANSI
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

	r.tasks[spanID] = &taskState{
		name:      name,
		startTime: startTime,
	}
	r.buffers[spanID] = new(bytes.Buffer)

	// Print start message to stderr
	prefix := r.output.String(fmt.Sprintf("[%s]", name)).Faint().String()
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
		r.printLineLocked(task.name, line)
	}
}

// OnTaskComplete flushes remaining buffer and prints completion status.
func (r *Renderer) OnTaskComplete(spanID string, endTime time.Time, err error) {
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
	prefix := fmt.Sprintf("[%s]", task.name)

	if err != nil {
		symbol := r.output.String("✗").Foreground(termenv.ANSIRed).String()
		_, _ = fmt.Fprintf(r.stderr, "%s %s Failed after %v: %v\n",
			prefix, symbol, duration, err)
	} else {
		symbol := r.output.String("✓").Foreground(termenv.ANSIGreen).String()
		_, _ = fmt.Fprintf(r.stderr, "%s %s Completed in %v\n",
			prefix, symbol, duration)
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
		r.printLineLocked(task.name, buf.Bytes())
		buf.Reset()
	}
}

// printLineLocked prints a line with the task name prefix.
// Must be called with r.mu held.
func (r *Renderer) printLineLocked(taskName string, line []byte) {
	// Trim trailing newline for cleaner output
	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))

	if len(line) == 0 {
		return
	}

	prefix := fmt.Sprintf("[%s]", taskName)
	_, _ = fmt.Fprintf(r.stdout, "%s %s\n", prefix, string(line))
}
