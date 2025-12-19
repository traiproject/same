package cas

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/core/ports"
)

const NodeID graft.ID = "adapter.build_info_store"

func init() {
	graft.Register(graft.Node[ports.BuildInfoStore]{
		ID:        NodeID,
		Cacheable: true,
		Run: func(ctx context.Context) (ports.BuildInfoStore, error) {
			store, err := NewStore()
			if err != nil {
				return nil, err
			}
			return store, nil
		},
	})
}
