package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.trai.ch/same/internal/adapters/telemetry"
)

// Renderer wraps the TUI Bubble Tea model as a ports.Renderer.
type Renderer struct {
	program *tea.Program
	model   *Model
	errCh   chan error
}

// NewRenderer creates a new TUI renderer.
func NewRenderer(model *Model, opts ...tea.ProgramOption) *Renderer {
	program := tea.NewProgram(model, opts...)
	return &Renderer{
		program: program,
		model:   model,
		errCh:   make(chan error, 1),
	}
}

// Start launches the TUI in a background goroutine.
func (r *Renderer) Start(_ context.Context) error {
	go func() {
		_, err := r.program.Run()
		r.errCh <- err
	}()
	return nil
}

// Stop signals the TUI to quit.
func (r *Renderer) Stop() error {
	r.program.Quit()
	return nil
}

// Wait blocks until the TUI has terminated.
func (r *Renderer) Wait() error {
	return <-r.errCh
}

// OnPlanEmit forwards plan initialization to the TUI.
func (r *Renderer) OnPlanEmit(tasks []string, deps map[string][]string, targets []string) {
	r.program.Send(telemetry.MsgInitTasks{
		Tasks:        tasks,
		Dependencies: deps,
		Targets:      targets,
	})
}

// OnTaskStart forwards task start events to the TUI.
func (r *Renderer) OnTaskStart(spanID, parentID, name string, startTime time.Time) {
	r.program.Send(telemetry.MsgTaskStart{
		SpanID:    spanID,
		ParentID:  parentID,
		Name:      name,
		StartTime: startTime,
	})
}

// OnTaskLog forwards task log data to the TUI.
func (r *Renderer) OnTaskLog(spanID string, data []byte) {
	r.program.Send(telemetry.MsgTaskLog{
		SpanID: spanID,
		Data:   data,
	})
}

// OnTaskComplete forwards task completion events to the TUI.
func (r *Renderer) OnTaskComplete(spanID string, endTime time.Time, err error, cached bool) {
	r.program.Send(telemetry.MsgTaskComplete{
		SpanID:  spanID,
		EndTime: endTime,
		Err:     err,
		Cached:  cached,
	})
}

// Program returns the underlying tea.Program for testing.
func (r *Renderer) Program() *tea.Program {
	return r.program
}
