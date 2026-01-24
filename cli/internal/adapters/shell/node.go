package shell

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/internal/core/ports"
)

// NodeID is the unique identifier for the executor Graft node.
const NodeID graft.ID = "adapter.executor"

func init() {
	graft.Register(graft.Node[ports.Executor]{
		ID:        NodeID,
		Cacheable: true,
		DependsOn: []graft.ID{},
		Run: func(_ context.Context) (ports.Executor, error) {
			return NewExecutor(), nil
		},
	})
}
