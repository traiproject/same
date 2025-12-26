package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.trai.ch/bob/internal/core/ports"
)

// OTelTracer is a concrete implementation of ports.Tracer using OpenTelemetry.
type OTelTracer struct {
	tracer trace.Tracer
}

// NewOTelTracer creates a new OTelTracer with the given instrumentation name.
func NewOTelTracer(name string) *OTelTracer {
	return &OTelTracer{
		tracer: otel.Tracer(name),
	}
}

// Start creates a new span.
func (t *OTelTracer) Start(ctx context.Context, name string, opts ...ports.SpanOption) (context.Context, ports.Span) {
	// Apply internal options to SpanConfig (currently placeholder)
	cfg := &ports.SpanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Start OTel span
	ctx, span := t.tracer.Start(ctx, name)

	return ctx, &OTelSpan{span: span}
}

// EmitPlan signals that a set of tasks is planned for execution by adding an event to the current span.
func (t *OTelTracer) EmitPlan(ctx context.Context, taskNames []string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("plan_emitted", trace.WithAttributes(
			attribute.StringSlice("tasks", taskNames),
		))
	}
}

// OTelSpan is a concrete implementation of ports.Span using OpenTelemetry.
type OTelSpan struct {
	span trace.Span
}

// End completes the span.
func (s *OTelSpan) End() {
	s.span.End()
}

// SetAttribute adds a key-value pair to the span.
func (s *OTelSpan) SetAttribute(key string, value any) {
	switch v := value.(type) {
	case string:
		s.span.SetAttributes(attribute.String(key, v))
	case int:
		s.span.SetAttributes(attribute.Int(key, v))
	case int64:
		s.span.SetAttributes(attribute.Int64(key, v))
	case float64:
		s.span.SetAttributes(attribute.Float64(key, v))
	case bool:
		s.span.SetAttributes(attribute.Bool(key, v))
	case []string:
		s.span.SetAttributes(attribute.StringSlice(key, v))
	default:
		s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
	}
}

// Write satisfies io.Writer by adding a log event to the span.
func (s *OTelSpan) Write(p []byte) (n int, err error) {
	s.span.AddEvent("log", trace.WithAttributes(attribute.String("message", string(p))))
	return len(p), nil
}
