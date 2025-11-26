package ports

import "go.trai.ch/bob/internal/core/domain"

// Hasher defines the interface for computing hashes.
//
//go:generate mockgen -destination=mocks/hasher_mock.go -package=mocks -source=hasher.go
type Hasher interface {
	// ComputeInputHash computes the input hash for a given task.
	ComputeInputHash(task *domain.Task, env map[string]string, rootDir string) (string, error)
}
