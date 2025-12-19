package config

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/adapters/logger"
	"go.trai.ch/bob/internal/core/ports"
)

const NodeID graft.ID = "adapter.config_loader"

func init() {
	graft.Register(graft.Node[ports.ConfigLoader]{
		ID:        NodeID,
		Cacheable: true,
		DependsOn: []graft.ID{logger.NodeID},
		Run: func(ctx context.Context) (ports.ConfigLoader, error) {
			log, err := graft.Dep[ports.Logger](ctx)
			if err != nil {
				return nil, err
			}
			return NewLoader(log), nil
		},
	})
}
