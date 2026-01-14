package logger

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/internal/core/ports"
)

// NodeID is the unique identifier for the logger Graft node.
const NodeID graft.ID = "adapter.logger"

func init() {
	graft.Register(graft.Node[ports.Logger]{
		ID:        NodeID,
		Cacheable: true,
		Run: func(_ context.Context) (ports.Logger, error) {
			return New(), nil
		},
	})
}
