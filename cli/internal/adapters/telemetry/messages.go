package telemetry

import (
	"time"
)

// MsgTaskStart indicates a new task (span) has started.
type MsgTaskStart struct {
	SpanID    string
	ParentID  string // May be empty if root
	Name      string
	StartTime time.Time
}

// MsgTaskComplete indicates a task (span) has finished.
type MsgTaskComplete struct {
	SpanID  string
	EndTime time.Time
	Err     error
}

// MsgTaskLog carries a chunk of log output for a specific task.
type MsgTaskLog struct {
	SpanID string
	Data   []byte
}

// MsgInitTasks serves as a signal to initialize or reset the task list in the UI.
type MsgInitTasks struct {
	Tasks        []string
	Dependencies map[string][]string
	Targets      []string
}
