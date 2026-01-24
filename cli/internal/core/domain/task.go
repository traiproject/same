package domain

// RebuildStrategy controls when a task should execute.
type RebuildStrategy string

const (
	// RebuildOnChange executes the task only when inputs have changed (default).
	RebuildOnChange RebuildStrategy = "on-change"

	// RebuildAlways executes the task on every run, bypassing the cache.
	RebuildAlways RebuildStrategy = "always"
)

// Task represents a unit of work in the build system.
// It uses InternedString for fields that are frequently repeated to save memory.
type Task struct {
	Name            InternedString
	Command         []string
	Inputs          []InternedString
	Outputs         []InternedString
	Tools           map[string]string
	Dependencies    []InternedString
	Environment     map[string]string
	WorkingDir      InternedString
	RebuildStrategy RebuildStrategy
}
