package nix

import "go.trai.ch/bob/internal/core/ports"

// NewEnvFactoryDefault creates a new EnvironmentFactory with the default cache directory.
// This is a convenience wrapper for dependency injection that uses the standard cache path.
func NewEnvFactoryDefault(
	resolver ports.DependencyResolver,
	manager ports.PackageManager,
) *EnvFactory {
	return NewEnvFactory(resolver, manager, ".bob/cache/environments")
}
