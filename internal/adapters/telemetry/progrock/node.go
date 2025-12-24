package progrock

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/bob/internal/core/ports"
)

const (
	// NodeID is the unique identifier for the telemetry adapter node.
	NodeID graft.ID = "adapter.telemetry"
)

func init() {
	graft.Register(graft.Node[ports.Telemetry]{
		ID:        NodeID,
		Cacheable: true,
		Run: func(_ context.Context) (ports.Telemetry, error) {
			return New(), nil
		},
	})
}
