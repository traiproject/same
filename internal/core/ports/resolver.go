package ports

import "context"

// InputResolver defines the interface for resolving input files.
//
//go:generate mockgen -destination=mocks/resolver_mock.go -package=mocks -source=resolver.go
type InputResolver interface {
	// ResolveInputs resolves the given input patterns to a list of concrete file paths.
	ResolveInputs(inputs []string, root string) ([]string, error)
}

// DependencyResolver defines the interface for resolving external dependencies (tools).
type DependencyResolver interface {
	// Resolve resolves a tool alias and version to a specific package reference (e.g. commit hash and attribute path).
	Resolve(ctx context.Context, alias, version string) (commitHash, attrPath string, err error)
}
