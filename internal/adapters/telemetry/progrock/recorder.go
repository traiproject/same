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
	tape *progrock.Tape
	rec  *progrock.Recorder
}

// New creates a new Recorder with a default tape.
func New() ports.Telemetry {
	tape := progrock.NewTape()
	rec := progrock.NewRecorder(tape)
	return &Recorder{
		tape: tape,
		rec:  rec,
	}
}

// Record starts recording a new vertex.
func (r *Recorder) Record(ctx context.Context, name string, _ ...ports.VertexOption) (context.Context, ports.Vertex) {
	// Note: We might want to apply VertexOptions here in the future if we need to configure the vertex.
	// For now, we just create a basic vertex on the tape.
	d := digest.FromString(name)
	v := r.rec.Vertex(d, name)
	return ctx, &Vertex{vertex: v}
}

// Close flushes and closes the recording session.
func (r *Recorder) Close() error {
	// Close the recorder first? Or just the tape?
	// The user requested: "Call r.tape.Close() to flush events."
	// We also probably want to close the recorder if it has any internal state flushing.
	// But minimal compliance is tape.Close().
	// Let's call basic tape.Close() as requested.
	return r.tape.Close()
}
