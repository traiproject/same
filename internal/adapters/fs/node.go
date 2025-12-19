package fs

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/core/ports"
)

const (
	WalkerNodeID   graft.ID = "adapter.fs.walker"
	ResolverNodeID graft.ID = "adapter.fs.resolver"
	HasherNodeID   graft.ID = "adapter.fs.hasher"
)

func init() {
	// Walker Node (Concrete implementation needed by Hasher)
	graft.Register(graft.Node[*Walker]{
		ID:        WalkerNodeID,
		Cacheable: true,
		Run: func(ctx context.Context) (*Walker, error) {
			return NewWalker(), nil
		},
	})

	// Resolver Node
	graft.Register(graft.Node[ports.InputResolver]{
		ID:        ResolverNodeID,
		Cacheable: true,
		Run: func(ctx context.Context) (ports.InputResolver, error) {
			return NewResolver(), nil
		},
	})

	// Hasher Node
	graft.Register(graft.Node[ports.Hasher]{
		ID:        HasherNodeID,
		Cacheable: true,
		DependsOn: []graft.ID{WalkerNodeID},
		Run: func(ctx context.Context) (ports.Hasher, error) {
			walker, err := graft.Dep[*Walker](ctx)
			if err != nil {
				return nil, err
			}
			return NewHasher(walker), nil
		},
	})
}
