package telemetry

import (
	"context"
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.trai.ch/same/internal/core/ports"
)

// LogBufferSize determines the size of the async log channel.
const LogBufferSize = 4096

// OTelTracer is a concrete implementation of ports.Tracer using OpenTelemetry.
type OTelTracer struct {
	tracer  trace.Tracer
	program *tea.Program
	logChan chan tea.Msg
	mu      sync.RWMutex
}

// NewOTelTracer creates a new OTelTracer with the given instrumentation name.
func NewOTelTracer(name string) *OTelTracer {
	t := &OTelTracer{
		tracer:  otel.Tracer(name),
		logChan: make(chan tea.Msg, LogBufferSize), // Buffered to handle bursts
	}
	go t.runLoop()
	return t
}

func (t *OTelTracer) runLoop() {
	for msg := range t.logChan {
		t.mu.RLock()
		prog := t.program
		t.mu.RUnlock()

		if prog != nil {
			prog.Send(msg)
		}
	}
}

// Shutdown stops the background log processor.
func (t *OTelTracer) Shutdown(_ context.Context) error {
	close(t.logChan)
	return nil
}

// WithProgram sets the tea.Program to send logs to.
func (t *OTelTracer) WithProgram(p *tea.Program) *OTelTracer {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.program = p
	return t
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

	t.mu.RLock()
	prog := t.program
	t.mu.RUnlock()

	var batcher *BatchProcessor
	if prog != nil {
		spanID := span.SpanContext().SpanID().String()
		cb := func(data []byte) {
			select {
			case t.logChan <- MsgTaskLog{
				SpanID: spanID,
				Data:   data,
			}:
			default:
				// Drop logs if buffer is full to prevent blocking the build
			}
		}
		// Use generic defaults or smaller limits for UI responsiveness?
		batcher = NewBatchProcessor(0, 0, cb)
	}

	return ctx, &OTelSpan{span: span, batcher: batcher}
}

// EmitPlan signals that a set of tasks is planned for execution by adding an event to the current span.
func (t *OTelTracer) EmitPlan(ctx context.Context, taskNames []string) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent("plan_emitted", trace.WithAttributes(
			attribute.StringSlice("tasks", taskNames),
		))
	}

	t.mu.RLock()
	prog := t.program
	t.mu.RUnlock()

	if prog != nil {
		select {
		case t.logChan <- MsgInitTasks{
			Tasks: taskNames,
		}:
		default:
			// Ensure InitTasks is sent even if buffer is full (blocking fallback)
			// This is critical for UI initialization
			t.logChan <- MsgInitTasks{
				Tasks: taskNames,
			}
		}
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
