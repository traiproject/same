package ports

import "go.trai.ch/bob/internal/core/domain"

// Hasher defines the interface for computing hashes of inputs and files.
type Hasher interface {
	// ComputeInputHash computes the hash of the task's input.
	ComputeInputHash(task *domain.Task, env map[string]string, root string) (string, error)

	// ComputeFileHash computes the hash of a file.
	ComputeFileHash(path string) (uint64, error)
}
