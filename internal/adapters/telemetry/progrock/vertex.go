package progrock

import (
	"fmt"
	"io"

	"github.com/vito/progrock"
	"go.trai.ch/bob/internal/core/domain"
)

// Vertex implements ports.Vertex wrapping *progrock.VertexRecorder.
type Vertex struct {
	vertex *progrock.VertexRecorder
}

// Stdout returns a writer to capture standard output stream.
func (v *Vertex) Stdout() io.Writer {
	return v.vertex.Stdout()
}

// Stderr returns a writer to capture error output stream.
func (v *Vertex) Stderr() io.Writer {
	return v.vertex.Stderr()
}

// Log records a structured log message associated with this vertex.
func (v *Vertex) Log(level domain.LogLevel, msg string) {
	_, _ = fmt.Fprintf(v.vertex.Stdout(), "[%s] %s\n", level.String(), msg)
}

// Complete marks the vertex as finished (successfully or with an error).
func (v *Vertex) Complete(err error) {
	v.vertex.Done(err)
}

// Cached marks the vertex as a cache hit.
func (v *Vertex) Cached() {
	v.vertex.Cached()
}
