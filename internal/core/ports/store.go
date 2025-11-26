package ports

import "go.trai.ch/bob/internal/core/domain"

// BuildInfoStore defines the interface for storing and retrieving build information.
//
//go:generate mockgen -source=store.go -destination=mocks/mock_store.go -package=mocks
type BuildInfoStore interface {
	// Get retrieves the build info for a given task name.
	// Returns nil, nil if not found.
	Get(taskName string) (*domain.BuildInfo, error)

	// Put stores the build info.
	Put(info domain.BuildInfo) error
}
