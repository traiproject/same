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

			return New(loader, sched), nil
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
		Run: func(ctx context.Context) (*Components, error) {
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

			// Manually construct Components to avoid casting issues with NewComponents
			// if it requires a concrete type.
			return &Components{
				App:          app,
				Logger:       log,
				configLoader: loader,
			}, nil
		},
	})
}
