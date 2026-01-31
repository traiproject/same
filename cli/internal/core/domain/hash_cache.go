package domain

import (
	"time"
	"unique"
)

// TaskHashEntry stores the computed hash and related metadata for a task.
type TaskHashEntry struct {
	// Hash is the computed input hash for the task.
	Hash string
	// ResolvedInputs is the list of resolved input paths at the time of hashing.
	ResolvedInputs []unique.Handle[string]
	// ComputedAt is the timestamp when the hash was computed.
	ComputedAt time.Time
}
