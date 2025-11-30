package ports

import (
	"context"
)

// Environment defines the interface for resolving system environment dependencies.
//
//go:generate mockgen -destination=mocks/environment_mock.go -package=mocks -source=environment.go
type Environment interface {
	// Resolve resolves the environment variables for the given system dependencies.
	// It returns a map of environment variables (e.g., PATH, CFLAGS) and an error if resolution fails.
	Resolve(ctx context.Context, dependencies []string) (map[string]string, error)
}
