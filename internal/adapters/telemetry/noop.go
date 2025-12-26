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
func (t *NoOpTracer) Start(ctx context.Context, name string, opts ...ports.SpanOption) (context.Context, ports.Span) {
	return ctx, &NoOpSpan{}
}

// EmitPlan does nothing.
func (t *NoOpTracer) EmitPlan(ctx context.Context, taskNames []string) {}

// NoOpSpan is a no-op implementation of ports.Span.
type NoOpSpan struct{}

// End does nothing.
func (s *NoOpSpan) End() {}

// SetAttribute does nothing.
func (s *NoOpSpan) SetAttribute(key string, value any) {}

// Write does nothing and returns the length of p.
func (s *NoOpSpan) Write(p []byte) (n int, err error) {
	return len(p), nil
}
