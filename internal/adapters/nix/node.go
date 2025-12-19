package nix

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/core/ports"
)

const (
	ResolverNodeID   graft.ID = "adapter.nix.resolver"
	EnvFactoryNodeID graft.ID = "adapter.nix.env_factory"
)

func init() {
	// Dependency Resolver Node
	graft.Register(graft.Node[ports.DependencyResolver]{
		ID:        ResolverNodeID,
		Cacheable: true,
		Run: func(ctx context.Context) (ports.DependencyResolver, error) {
			resolver, err := NewResolver()
			if err != nil {
				return nil, err
			}
			return resolver, nil
		},
	})

	// Environment Factory Node
	graft.Register(graft.Node[ports.EnvironmentFactory]{
		ID:        EnvFactoryNodeID,
		Cacheable: true,
		DependsOn: []graft.ID{ResolverNodeID},
		Run: func(ctx context.Context) (ports.EnvironmentFactory, error) {
			resolver, err := graft.Dep[ports.DependencyResolver](ctx)
			if err != nil {
				return nil, err
			}
			return NewEnvFactory(resolver), nil
		},
	})
}
