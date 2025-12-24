// Package progrock provides the Progrock implementation of the telemetry adapter.
package progrock

import (
	"context"

	"go.trai.ch/bob/internal/core/ports"
)

// Recorder implements the ports.Telemetry interface using the apps/progrock library.
type Recorder struct{}

// Record starts recording a new vertex.
func (r *Recorder) Record(ctx context.Context, name string, opts ...ports.VertexOption) (context.Context, ports.Vertex) {
	panic("not implemented")
}

// Close flushes and closes the recording session.
func (r *Recorder) Close() error {
	panic("not implemented")
}
