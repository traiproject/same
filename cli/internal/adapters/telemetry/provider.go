package telemetry

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.trai.ch/same/internal/core/ports"
)

// OTelTracer is a concrete implementation of ports.Tracer using OpenTelemetry.
type OTelTracer struct {
	tracer   trace.Tracer
	renderer ports.Renderer
	mu       sync.RWMutex
}

// NewOTelTracer creates a new OTelTracer with the given instrumentation name.
func NewOTelTracer(name string) *OTelTracer {
	return &OTelTracer{
		tracer: otel.Tracer(name),
	}
}

// Shutdown stops the tracer.
func (t *OTelTracer) Shutdown(_ context.Context) error {
	return nil
}

// WithRenderer sets the Renderer to send logs to.
func (t *OTelTracer) WithRenderer(r ports.Renderer) *OTelTracer {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.renderer = r
	return t
}

// Start creates a new span.
func (t *OTelTracer) Start(ctx context.Context, name string, opts ...ports.SpanOption) (context.Context, ports.Span) {
	cfg := &ports.SpanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, span := t.tracer.Start(ctx, name)

	t.mu.RLock()
	r := t.renderer
	t.mu.RUnlock()

	var batcher *BatchProcessor
	if r != nil {
		spanID := span.SpanContext().SpanID().String()
		cb := func(data []byte) {
			r.OnTaskLog(spanID, data)
		}
		batcher = NewBatchProcessor(0, 0, cb)
	}

	return ctx, &OTelSpan{span: span, batcher: batcher}
}

// EmitPlan signals that a set of tasks is planned for execution.
func (t *OTelTracer) EmitPlan(
	ctx context.Context,
	taskNames []string,
	dependencies map[string][]string,
	targets []string,
) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("plan_emitted", trace.WithAttributes(
			attribute.StringSlice("tasks", taskNames),
		))
	}

	t.mu.RLock()
	r := t.renderer
	t.mu.RUnlock()

	if r != nil {
		r.OnPlanEmit(taskNames, dependencies, targets)
	}
}

// OTelSpan is a concrete implementation of ports.Span using OpenTelemetry.
type OTelSpan struct {
	span    trace.Span
	batcher *BatchProcessor
}

// End completes the span.
func (s *OTelSpan) End() {
	if s.batcher != nil {
		_ = s.batcher.Close()
	}
	s.span.End()
}

// RecordError records an error for the span.
func (s *OTelSpan) RecordError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
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

// Write satisfies io.Writer by adding a log event to the span or writing to the batcher.
func (s *OTelSpan) Write(p []byte) (n int, err error) {
	if s.batcher != nil {
		return s.batcher.Write(p)
	}
	s.span.AddEvent("log", trace.WithAttributes(attribute.String("message", string(p))))
	return len(p), nil
}

// MarkExecStart signals that command execution has begun.
func (s *OTelSpan) MarkExecStart() {
	s.span.AddEvent("exec_start")
}
