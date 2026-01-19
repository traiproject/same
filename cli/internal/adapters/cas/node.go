package cas

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/internal/core/ports"
)

// NodeID is the unique identifier for the build info store Graft node.
const NodeID graft.ID = "adapter.build_info_store"

func init() {
	graft.Register(graft.Node[ports.BuildInfoStore]{
		ID:        NodeID,
		Cacheable: true,
		Run: func(_ context.Context) (ports.BuildInfoStore, error) {
			store, err := NewStore()
			if err != nil {
				return nil, err
			}
			return store, nil
		},
	})
}
