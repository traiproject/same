package nix

import "net/http"

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
