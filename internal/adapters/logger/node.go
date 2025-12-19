package logger

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/core/ports"
)

const NodeID graft.ID = "adapter.logger"

func init() {
	graft.Register(graft.Node[ports.Logger]{
		ID:        NodeID,
		Cacheable: true,
		Run: func(ctx context.Context) (ports.Logger, error) {
			return New(), nil
		},
	})
}
