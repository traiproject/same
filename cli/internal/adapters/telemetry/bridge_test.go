package telemetry_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestBridge_OnStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRenderer := mocks.NewMockRenderer(ctrl)
	bridge := telemetry.NewBridge(mockRenderer)

	mockRenderer.EXPECT().OnTaskStart(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	if rwSpan, ok := span.(sdktrace.ReadWriteSpan); ok {
		bridge.OnStart(ctx, rwSpan)
	}
}

func TestBridge_OnStartWithNilRenderer(_ *testing.T) {
	bridge := telemetry.NewBridge(nil)

	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	if rwSpan, ok := span.(sdktrace.ReadWriteSpan); ok {
		bridge.OnStart(ctx, rwSpan)
	}
}

func TestBridge_OnEnd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRenderer := mocks.NewMockRenderer(ctrl)
	bridge := telemetry.NewBridge(mockRenderer)

	mockRenderer.EXPECT().OnTaskComplete(gomock.Any(), gomock.Any(), nil).Times(1)

	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	if roSpan, ok := span.(sdktrace.ReadOnlySpan); ok {
		bridge.OnEnd(roSpan)
	}
}

func TestBridge_OnEndWithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRenderer := mocks.NewMockRenderer(ctrl)
	bridge := telemetry.NewBridge(mockRenderer)

	mockRenderer.EXPECT().OnTaskComplete(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	span.SetStatus(codes.Error, "test error")
	span.End()

	if roSpan, ok := span.(sdktrace.ReadOnlySpan); ok {
		bridge.OnEnd(roSpan)
	}
}

func TestBridge_OnEndWithNilRenderer(_ *testing.T) {
	bridge := telemetry.NewBridge(nil)

	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	if roSpan, ok := span.(sdktrace.ReadOnlySpan); ok {
		bridge.OnEnd(roSpan)
	}
}

func TestBridge_ForceFlush(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRenderer := mocks.NewMockRenderer(ctrl)
	bridge := telemetry.NewBridge(mockRenderer)

	if err := bridge.ForceFlush(context.Background()); err != nil {
		t.Errorf("ForceFlush() should not return error, got: %v", err)
	}
}

func TestBridge_Shutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRenderer := mocks.NewMockRenderer(ctrl)
	bridge := telemetry.NewBridge(mockRenderer)

	if err := bridge.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown() should not return error, got: %v", err)
	}
}
