package nix

import "go.trai.ch/same/internal/core/ports"

// NewEnvFactory creates a new EnvironmentFactory with the default cache directory.
// This is a convenience wrapper for dependency injection that uses the standard cache path.
func NewEnvFactory(
	resolver ports.DependencyResolver,
) *EnvFactory {
	return NewEnvFactoryWithCache(resolver, ".bob/cache/environments")
}
