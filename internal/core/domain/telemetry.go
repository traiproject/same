package domain

import "strings"

// VertexStatus represents the lifecycle state of a unit of work (Vertex) in the build graph.
type VertexStatus string

const (
	// VertexStatusPending indicates the vertex is waiting for dependencies or scheduling.
	VertexStatusPending VertexStatus = "pending"
	// VertexStatusRunning indicates the vertex is currently executing.
	VertexStatusRunning VertexStatus = "running"
	// VertexStatusCompleted indicates the vertex executed successfully.
	VertexStatusCompleted VertexStatus = "completed"
	// VertexStatusFailed indicates the vertex execution failed.
	VertexStatusFailed VertexStatus = "failed"
	// VertexStatusCached indicates the vertex work was skipped because a valid cache was found.
	VertexStatusCached VertexStatus = "cached"
	// VertexStatusSkipped indicates the vertex was skipped for reasons other than caching (e.g. conditional execution).
	VertexStatusSkipped VertexStatus = "skipped"
)

// LogLevel represents the severity of a log message, mirroring the standard slog levels.
type LogLevel int

const (
	// LogLevelDebug represents debug-level verbosity.
	LogLevelDebug LogLevel = -4
	// LogLevelInfo represents informational verbosity.
	LogLevelInfo LogLevel = 0
	// LogLevelWarn represents warning verbosity.
	LogLevelWarn LogLevel = 4
	// LogLevelError represents error verbosity.
	LogLevelError LogLevel = 8
)

// String returns the string representation of the LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// IsTerminal checks if a status is a terminal state (Completed, Failed, Cached, Skipped).
func (s VertexStatus) IsTerminal() bool {
	switch s {
	case VertexStatusCompleted, VertexStatusFailed, VertexStatusCached, VertexStatusSkipped:
		return true
	default:
		return false
	}
}

// NormalizeVertexStatus converts a string to a VertexStatus, defaulting to pending if unknown.
// This is useful for deserialization or API boundaries.
func NormalizeVertexStatus(s string) VertexStatus {
	switch strings.ToLower(s) {
	case string(VertexStatusPending):
		return VertexStatusPending
	case string(VertexStatusRunning):
		return VertexStatusRunning
	case string(VertexStatusCompleted):
		return VertexStatusCompleted
	case string(VertexStatusFailed):
		return VertexStatusFailed
	case string(VertexStatusCached):
		return VertexStatusCached
	case string(VertexStatusSkipped):
		return VertexStatusSkipped
	default:
		return VertexStatusPending
	}
}
