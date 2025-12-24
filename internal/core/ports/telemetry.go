package ports

import (
	"context"
	"io"

	"go.trai.ch/bob/internal/core/domain"
)

// Vertex represents a single unit of work (e.g., a build task or setup step)
// in the build graph that can be recorded.
type Vertex interface {
	// Stdout returns a writer to capture standard output stream.
	Stdout() io.Writer
	// Stderr returns a writer to capture error output stream.
	Stderr() io.Writer
	// Log records a structured log message associated with this vertex.
	Log(level domain.LogLevel, msg string)
	// Complete marks the vertex as finished (successfully or with an error).
	Complete(err error)
	// Cached marks the vertex as a cache hit.
	Cached()
}

type vertexKey struct{}

// ContextWithVertex returns a new context with the given Vertex embedded.
func ContextWithVertex(ctx context.Context, v Vertex) context.Context {
	return context.WithValue(ctx, vertexKey{}, v)
}

// VertexFromContext retrieves the Vertex from the context, if present.
func VertexFromContext(ctx context.Context) (Vertex, bool) {
	v, ok := ctx.Value(vertexKey{}).(Vertex)
	return v, ok
}

// VertexOption is a configuration function for creating a Vertex.
type VertexOption func(Vertex)

// Telemetry is the factory/manager for recording build events.
//
//go:generate go run go.uber.org/mock/mockgen -source=telemetry.go -destination=mocks/mock_telemetry.go -package=mocks
type Telemetry interface {
	// Record starts recording a new vertex.
	Record(ctx context.Context, name string, opts ...VertexOption) (context.Context, Vertex)
	// Close flushes and closes the recording session.
	Close() error
}
