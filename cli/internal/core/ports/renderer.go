package ports

import (
	"context"
	"time"
)

// Renderer is the abstraction for output rendering.
// It decouples telemetry collection from presentation logic,
// allowing the same event stream to drive either a rich TUI or linear CI logs.
//
//go:generate mockgen -source=renderer.go -destination=mocks/mock_renderer.go -package=mocks
type Renderer interface {
	// Start initializes the renderer and begins its lifecycle.
	// For asynchronous renderers (like TUI), this may launch background goroutines.
	Start(ctx context.Context) error

	// Stop signals the renderer to stop accepting new events and prepare for shutdown.
	// It should flush any buffered output.
	Stop() error

	// Wait blocks until the renderer has fully terminated.
	// For synchronous renderers, this may return immediately.
	Wait() error

	// OnPlanEmit is called when the scheduler has planned the task graph.
	// tasks: list of all task names in execution order
	// deps: dependency map (task -> list of dependencies)
	// targets: the user-requested target tasks
	OnPlanEmit(tasks []string, deps map[string][]string, targets []string)

	// OnTaskStart is called when a task begins execution.
	// spanID: unique identifier for this task execution
	// parentID: spanID of the parent task (empty if root)
	// name: human-readable task name
	// startTime: when the task started
	OnTaskStart(spanID, parentID, name string, startTime time.Time)

	// OnTaskLog is called when a task emits output.
	// spanID: identifier for the task
	// data: raw log bytes (may contain partial lines or ANSI sequences)
	OnTaskLog(spanID string, data []byte)

	// OnTaskComplete is called when a task finishes execution.
	// spanID: identifier for the task
	// endTime: when the task completed
	// err: nil if successful, error otherwise
	OnTaskComplete(spanID string, endTime time.Time, err error)
}
