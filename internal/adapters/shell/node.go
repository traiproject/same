package shell

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/adapters/logger"
	"go.trai.ch/bob/internal/core/ports"
)

const NodeID graft.ID = "adapter.executor"

func init() {
	graft.Register(graft.Node[ports.Executor]{
		ID:        NodeID,
		Cacheable: true,
		DependsOn: []graft.ID{logger.NodeID},
		Run: func(ctx context.Context) (ports.Executor, error) {
			log, err := graft.Dep[ports.Logger](ctx)
			if err != nil {
				return nil, err
			}
			return NewExecutor(log), nil
		},
	})
}
