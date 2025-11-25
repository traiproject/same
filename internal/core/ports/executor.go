// Package ports defines the core interfaces for the application.
package ports

import (
	"context"

	"go.trai.ch/bob/internal/core/domain"
)

// Executor defines the interface for executing tasks.
//
//go:generate mockgen -source=executor.go -destination=mocks/mock_executor.go -package=mocks
type Executor interface {
	// Execute runs the given task.
	// It returns an error if the task execution fails.
	Execute(ctx context.Context, task *domain.Task) error
}
