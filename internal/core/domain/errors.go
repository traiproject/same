package domain

import "go.trai.ch/zerr"

var (
	// ErrTaskAlreadyExists is returned when attempting to add a task with a name that already exists.
	ErrTaskAlreadyExists = zerr.New("task already exists")

	// ErrMissingDependency is returned when a task references a dependency that doesn't exist in the graph.
	ErrMissingDependency = zerr.New("missing dependency")

	// ErrCycleDetected is returned when a cycle is detected in the task dependency graph.
	ErrCycleDetected = zerr.New("cycle detected")
)
