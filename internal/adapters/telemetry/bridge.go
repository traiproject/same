package telemetry

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// TUIBridge implements sdktrace.SpanProcessor to bridge OTel spans to Bubble Tea messages.
type TUIBridge struct {
	program *tea.Program
}

// NewTUIBridge returns a new TUIBridge.
func NewTUIBridge(program *tea.Program) *TUIBridge {
	return &TUIBridge{
		program: program,
	}
}

// OnStart is called when a span starts.
func (b *TUIBridge) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if b.program == nil {
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

	b.program.Send(MsgTaskStart{
		SpanID:    sc.SpanID().String(),
		ParentID:  parentID,
		Name:      s.Name(),
		StartTime: s.StartTime(),
	})
}

// OnEnd is called when a span ends.
func (b *TUIBridge) OnEnd(s sdktrace.ReadOnlySpan) {
	if b.program == nil {
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

	b.program.Send(MsgTaskComplete{
		SpanID:  sc.SpanID().String(),
		EndTime: s.EndTime(),
		Err:     err,
	})
}

// ForceFlush does nothing.
func (b *TUIBridge) ForceFlush(ctx context.Context) error {
	return nil
}

// Shutdown does nothing.
func (b *TUIBridge) Shutdown(ctx context.Context) error {
	return nil
}
