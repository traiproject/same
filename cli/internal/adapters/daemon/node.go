package daemon

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/internal/core/ports"
)

// NodeID is the unique identifier for the daemon connector Graft node.
const NodeID graft.ID = "adapter.daemon"

func init() {
	graft.Register(graft.Node[ports.DaemonConnector]{
		ID:        NodeID,
		Cacheable: true,
		Run: func(_ context.Context) (ports.DaemonConnector, error) {
			return NewConnector()
		},
	})
}
