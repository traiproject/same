package app

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/adapters/config" //nolint:depguard // Wired in app layer
	"go.trai.ch/bob/internal/adapters/logger" //nolint:depguard // Wired in app layer
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/bob/internal/engine/scheduler"
)

const (
	// AppNodeID is the unique identifier for the main App Graft node.
	AppNodeID graft.ID = "app.main"
	// ComponentsNodeID is the unique identifier for the App components Graft node.
	ComponentsNodeID graft.ID = "app.components"
)

func init() {
	// App Node
	graft.Register(graft.Node[*App]{
		ID:        AppNodeID,
		Cacheable: true,
		DependsOn: []graft.ID{
			config.NodeID,
			scheduler.NodeID,
		},
		Run: func(ctx context.Context) (*App, error) {
			loader, err := graft.Dep[ports.ConfigLoader](ctx)
			if err != nil {
				return nil, err
			}

			sched, err := graft.Dep[*scheduler.Scheduler](ctx)
			if err != nil {
				return nil, err
			}

			telemetry, err := graft.Dep[ports.Telemetry](ctx)
			if err != nil {
				return nil, err
			}

			return New(loader, sched, telemetry), nil
		},
	})

	// Components Node
	graft.Register(graft.Node[*Components]{
		ID:        ComponentsNodeID,
		Cacheable: true,
		DependsOn: []graft.ID{
			AppNodeID,
			logger.NodeID,
			config.NodeID,
		},
		Run: runComponentsNode,
	})
}

func runComponentsNode(ctx context.Context) (*Components, error) {
	app, err := graft.Dep[*App](ctx)
	if err != nil {
		return nil, err
	}

	log, err := graft.Dep[ports.Logger](ctx)
	if err != nil {
		return nil, err
	}

	loader, err := graft.Dep[ports.ConfigLoader](ctx)
	if err != nil {
		return nil, err
	}

	executor, err := graft.Dep[ports.Executor](ctx)
	if err != nil {
		return nil, err
	}

	store, err := graft.Dep[ports.BuildInfoStore](ctx)
	if err != nil {
		return nil, err
	}

	hasher, err := graft.Dep[ports.Hasher](ctx)
	if err != nil {
		return nil, err
	}

	resolver, err := graft.Dep[ports.InputResolver](ctx)
	if err != nil {
		return nil, err
	}

	envFactory, err := graft.Dep[ports.EnvironmentFactory](ctx)
	if err != nil {
		return nil, err
	}

	// Manually construct Components to avoid casting issues with NewComponents
	// if it requires a concrete type.
	return &Components{
		App:          app,
		Logger:       log,
		ConfigLoader: loader,
		Executor:     executor,
		Store:        store,
		Hasher:       hasher,
		Resolver:     resolver,
		EnvFactory:   envFactory,
	}, nil
}
