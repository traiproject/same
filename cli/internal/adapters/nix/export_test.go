package nix

import (
	"context"
	"net/http"
	"os/exec"
)

// Export constants and functions for testing purposes.

var GetCurrentSystem = getCurrentSystem

func NewResolverWithClient(path string, client *http.Client) (*Resolver, error) {
	return newResolverWithClient(path, client)
}

// Export types for testing purposes by aliasing them.
// We can't verify unexported types in external tests easily without this.
type (
	NixHubResponse = nixHubResponse
)

func (e *EnvFactory) GenerateNixExprRaw(system string, commits map[string][]string) string {
	return e.generateNixExpr(system, commits)
}

// SetExecCommandContext allows mocking exec.CommandContext.
// It returns a restore function.
func SetExecCommandContext(f func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	original := execCommandContext
	execCommandContext = f
	return func() {
		execCommandContext = original
	}
}
