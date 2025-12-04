package ports

import "context"

// DependencyResolver handles resolving a tool version to a specific Nixpkgs commit.
type DependencyResolver interface {
	// Resolve resolves a package identifier (e.g., "go@1.21") to a Nixpkgs commit hash.
	// It should check the cache first, then query the NixHub API.
	Resolve(ctx context.Context, alias, version string) (commitHash string, err error)
}

// PackageManager handles the fetching and preparation of tools.
type PackageManager interface {
	// Install ensures the tool from the specific commit is available in the Nix store.
	// Returns the absolute path to the tool's binary or environment.
	Install(ctx context.Context, toolName, commitHash string) (storePath string, err error)
}
