package scheduler

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/adapters/cas"       //nolint:depguard // Wired in engine wiring
	"go.trai.ch/bob/internal/adapters/fs"        //nolint:depguard // Wired in engine wiring
	"go.trai.ch/bob/internal/adapters/nix"       //nolint:depguard // Wired in engine wiring
	"go.trai.ch/bob/internal/adapters/shell"     //nolint:depguard // Wired in engine wiring
	"go.trai.ch/bob/internal/adapters/telemetry" //nolint:depguard // Wired in engine wiring
	"go.trai.ch/bob/internal/core/ports"
)

// NodeID is the unique identifier for the scheduler Graft node.
const NodeID graft.ID = "engine.scheduler"

func init() {
	graft.Register(graft.Node[*Scheduler]{
		ID:        NodeID,
		Cacheable: true,
		DependsOn: []graft.ID{
			shell.NodeID,
			cas.NodeID,
			fs.HasherNodeID,
			fs.ResolverNodeID,
			telemetry.TracerNodeID,
			nix.EnvFactoryNodeID,
		},
		Run: func(ctx context.Context) (*Scheduler, error) {
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

			tracer, err := graft.Dep[ports.Tracer](ctx)
			if err != nil {
				return nil, err
			}

			envFactory, err := graft.Dep[ports.EnvironmentFactory](ctx)
			if err != nil {
				return nil, err
			}

			return NewScheduler(
				executor,
				store,
				hasher,
				resolver,
				tracer,
				envFactory,
			), nil
		},
	})
}
