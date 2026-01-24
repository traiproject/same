package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.trai.ch/same/internal/core/ports"
)

// Bridge implements sdktrace.SpanProcessor to bridge OTel spans to a Renderer.
type Bridge struct {
	renderer ports.Renderer
}

// NewBridge returns a new Bridge.
func NewBridge(renderer ports.Renderer) *Bridge {
	return &Bridge{
		renderer: renderer,
	}
}

// OnStart is called when a span starts.
func (b *Bridge) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if b.renderer == nil {
		return
	}

	sc := s.SpanContext()
	if !sc.IsValid() {
		return
	}

	var parentID string
	if parentSpan := trace.SpanFromContext(parent); parentSpan.SpanContext().IsValid() {
		parentID = parentSpan.SpanContext().SpanID().String()
	}

	b.renderer.OnTaskStart(
		sc.SpanID().String(),
		parentID,
		s.Name(),
		s.StartTime(),
	)
}

// OnEnd is called when a span ends.
func (b *Bridge) OnEnd(s sdktrace.ReadOnlySpan) {
	if b.renderer == nil {
		return
	}

	sc := s.SpanContext()
	if !sc.IsValid() {
		return
	}

	var err error
	if s.Status().Code == codes.Error {
		desc := s.Status().Description
		if desc == "" {
			desc = "task failed"
		}
		err = errors.New(desc)
	}

	b.renderer.OnTaskComplete(
		sc.SpanID().String(),
		s.EndTime(),
		err,
	)
}

// ForceFlush does nothing.
func (b *Bridge) ForceFlush(_ context.Context) error {
	return nil
}

// Shutdown does nothing.
func (b *Bridge) Shutdown(_ context.Context) error {
	return nil
}
