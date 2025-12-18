// Package ports defines the core interfaces for the application.
package ports

import (
	"context"
)

// EnvironmentFactory creates hermetic execution environments from tool specifications.
//
// Implementations are responsible for:
//   - Resolving tool specifications (e.g., "go@1.25.4") to concrete packages
//   - Installing/preparing the required tools
//   - Constructing environment variables (PATH, GOROOT, etc.) for hermetic execution
//
//go:generate go run go.uber.org/mock/mockgen -source=environment.go -destination=mocks/mock_environment.go -package=mocks
type EnvironmentFactory interface {
	// GetEnvironment constructs a hermetic environment from a set of tools.
	//
	// The tools map contains alias->spec pairs (e.g., "go" -> "go@1.25.4").
	// Returns environment variables as "KEY=VALUE" strings suitable for process execution.
	//
	// The returned environment should include all necessary variables for the tools to function
	// correctly (e.g., PATH containing tool binaries, language-specific variables like GOROOT).
	//
	// Returns an error if any tool cannot be resolved or prepared.
	GetEnvironment(ctx context.Context, tools map[string]string) ([]string, error)
}
