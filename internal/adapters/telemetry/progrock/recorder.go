// Package progrock provides the Progrock implementation of the telemetry adapter.
package progrock

import (
	"context"

	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock"
	"go.trai.ch/bob/internal/core/ports"
)

// Recorder implements the ports.Telemetry interface using the apps/progrock library.
type Recorder struct {
	w   progrock.Writer
	rec *progrock.Recorder
}

// New creates a new Recorder with a default tape.
func New() ports.Telemetry {
	tape := progrock.NewTape()
	return NewRecorder(tape)
}

// NewRecorder creates a new Recorder with the given writer.
func NewRecorder(w progrock.Writer) *Recorder {
	rec := progrock.NewRecorder(w)
	return &Recorder{
		w:   w,
		rec: rec,
	}
}

// Record starts recording a new vertex.
func (r *Recorder) Record(ctx context.Context, name string, _ ...ports.VertexOption) (context.Context, ports.Vertex) {
	// Note: We might want to apply VertexOptions here in the future if we need to configure the vertex.
	// For now, we just create a basic vertex on the tape.
	d := digest.FromString(name)
	v := r.rec.Vertex(d, name)
	vertex := &Vertex{vertex: v}
	return ports.ContextWithVertex(ctx, vertex), vertex
}

// Close flushes and closes the recording session.
func (r *Recorder) Close() error {
	// If the writer implements Close, call it.
	if c, ok := r.w.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
