package watcher

import (
	"context"
	"time"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/internal/adapters/fs"
	"go.trai.ch/same/internal/core/ports"
)

const (
	// WatcherNodeID is the unique identifier for the file watcher Graft node.
	WatcherNodeID graft.ID = "adapter.watcher"
	// HashCacheNodeID is the unique identifier for the input hash cache Graft node.
	HashCacheNodeID graft.ID = "adapter.hash_cache"
)

func init() {
	// Watcher Node
	graft.Register(graft.Node[ports.Watcher]{
		ID:        WatcherNodeID,
		Cacheable: true,
		Run: func(_ context.Context) (ports.Watcher, error) {
			return NewWatcher()
		},
	})

	// HashCache Node
	graft.Register(graft.Node[*HashCache]{
		ID:        HashCacheNodeID,
		Cacheable: true,
		DependsOn: []graft.ID{fs.HasherNodeID, fs.ResolverNodeID},
		Run: func(ctx context.Context) (*HashCache, error) {
			hasher, err := graft.Dep[ports.Hasher](ctx)
			if err != nil {
				return nil, err
			}
			resolver, err := graft.Dep[ports.InputResolver](ctx)
			if err != nil {
				return nil, err
			}
			// The actual environment and root will come from the daemon's runtime context.
			return NewHashCache(hasher, resolver), nil
		},
	})
}

// NodeID returns the Graft node ID for a given port interface type.
// This is a helper to map port types to their corresponding node IDs.
func NodeID(portType any) graft.ID {
	switch portType.(type) {
	case ports.Watcher:
		return WatcherNodeID
	case *HashCache:
		return HashCacheNodeID
	default:
		// This is a compile-time check to ensure the type is handled.
		// If you get a compilation error here, add the new port type to the switch.
		panic("unknown port type")
	}
}

// DefaultDebounceWindow is the default time window for debouncing file events.
const DefaultDebounceWindow = 50 * time.Millisecond
