package telemetry

import (
	"context"

	"github.com/grindlemire/graft"
	"go.trai.ch/same/internal/core/ports"
)

// TracerNodeID is the unique identifier for the Telemetry adapter Graft node.
const TracerNodeID graft.ID = "adapter.telemetry"

func init() {
	graft.Register(graft.Node[ports.Tracer]{
		ID:        TracerNodeID,
		Cacheable: true,
		Run: func(_ context.Context) (ports.Tracer, error) {
			return NewOTelTracer("bob"), nil
		},
	})
}
