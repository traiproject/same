package telemetry

import (
	"context"

	"go.trai.ch/bob/internal/core/ports"
)

// NoOpTracer is a no-op implementation of ports.Tracer.
type NoOpTracer struct{}

// NewNoOpTracer creates a new NoOpTracer.
func NewNoOpTracer() *NoOpTracer {
	return &NoOpTracer{}
}

// Start creates a new no-op span.
// Start creates a new no-op span.
func (t *NoOpTracer) Start(ctx context.Context, _ string, _ ...ports.SpanOption) (context.Context, ports.Span) {
	return ctx, &NoOpSpan{}
}

// EmitPlan does nothing.
func (t *NoOpTracer) EmitPlan(_ context.Context, _ []string) {}

// NoOpSpan is a no-op implementation of ports.Span.
type NoOpSpan struct{}

// End does nothing.
func (s *NoOpSpan) End() {}

// SetAttribute does nothing.
func (s *NoOpSpan) SetAttribute(_ string, _ any) {}

// Write does nothing and returns the length of p.
func (s *NoOpSpan) Write(p []byte) (n int, err error) {
	return len(p), nil
}
