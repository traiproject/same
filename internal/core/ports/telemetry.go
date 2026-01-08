package ports

import (
	"context"
	"io"
)

//go:generate mockgen -source=telemetry.go -destination=mocks/mock_telemetry.go -package=mocks

// Tracer is the entry point for creating spans.
type Tracer interface {
	// Start creates a new span.
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
	// EmitPlan signals that a set of tasks is planned for execution.
	EmitPlan(ctx context.Context, taskNames []string)
}

// Span represents a unit of work.
type Span interface {
	io.Writer
	// End completes the span.
	End()
	// RecordError records an error for the span.
	RecordError(err error)
	// SetAttribute adds a key-value pair to the span.
	SetAttribute(key string, value any)
}

// SpanConfig holds configuration for a starting span.
type SpanConfig struct {
	// Add potential future configuration fields here.
	// For now, it's a placeholder to support the option pattern.
}

// SpanOption is a functional option for configuring a span.
type SpanOption func(*SpanConfig)
