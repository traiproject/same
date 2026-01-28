package ports

// InputHashState represents the state of an input hash computation.
type InputHashState uint8

const (
	// HashReady indicates the hash has been computed and is available.
	HashReady InputHashState = iota
	// HashPending indicates the hash is currently being computed.
	HashPending
	// HashUnknown indicates the hash state is unknown (typically means not yet cached).
	HashUnknown
)

// InputHashResult contains the result of an input hash query.
type InputHashResult struct {
	// State indicates the current state of the hash computation.
	State InputHashState
	// Hash is the computed hash (only valid when State is HashReady).
	Hash string
}

// InputHashCache defines the interface for caching and managing input hashes.
//
//go:generate mockgen -destination=mocks/mock_input_hash_cache.go -package=mocks -source=input_hash_cache.go
type InputHashCache interface {
	// GetInputHash returns the current hash state and value for the given task.
	// root and env are provided per-request to avoid race conditions when multiple
	// requests query hashes for the same task in different contexts.
	// It returns HashUnknown if the task has not been cached yet.
	GetInputHash(taskName, root string, env map[string]string) InputHashResult
	// Invalidate marks cached hashes for tasks affected by the changed paths.
	// This should be called when files are modified.
	Invalidate(paths []string)
}
