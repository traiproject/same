package telemetry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.trai.ch/same/internal/adapters/telemetry"
)

func TestOTelTracer_WithRenderer(t *testing.T) {
	mock := &mockRenderer{}
	tracer := telemetry.NewOTelTracer("test-tracer").WithRenderer(mock)
	ctx := context.Background()

	tracer.EmitPlan(ctx, []string{"task1"}, map[string][]string{}, []string{})

	mock.mu.Lock()
	planCalls := mock.planCalls
	mock.mu.Unlock()
	assert.Equal(t, 1, planCalls)

	_, span := tracer.Start(ctx, "test-span")
	_, err := span.Write([]byte("log data"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	mock.mu.Lock()
	logCalls := mock.logCalls
	mock.mu.Unlock()
	assert.Positive(t, logCalls)

	span.End()
}

func TestBridge(t *testing.T) {
	mock := &mockRenderer{}
	bridge := telemetry.NewBridge(mock)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bridge))
	tracer := tp.Tracer("test-bridge")

	_, span := tracer.Start(context.Background(), "test-task")

	time.Sleep(10 * time.Millisecond)
	mock.mu.Lock()
	startCalls := mock.startCalls
	mock.mu.Unlock()
	assert.Equal(t, 1, startCalls)

	span.End()

	time.Sleep(10 * time.Millisecond)
	mock.mu.Lock()
	completeCalls := mock.completeCalls
	mock.mu.Unlock()
	assert.Equal(t, 1, completeCalls)

	_, spanErr := tracer.Start(context.Background(), "test-error")
	time.Sleep(10 * time.Millisecond)

	spanErr.RecordError(errors.New("some error"))
	spanErr.SetStatus(codes.Error, "task failed explicitly")
	spanErr.End()

	time.Sleep(10 * time.Millisecond)
	mock.mu.Lock()
	completeCalls = mock.completeCalls
	mock.mu.Unlock()
	assert.Equal(t, 2, completeCalls)
}

func TestOTelSpan_Attributes(_ *testing.T) {
	tracer := telemetry.NewOTelTracer("test")
	_, span := tracer.Start(context.Background(), "test")

	span.SetAttribute("string", "val")
	span.SetAttribute("int", 123)
	span.SetAttribute("int64", int64(123))
	span.SetAttribute("float64", 12.34)
	span.SetAttribute("bool", true)
	span.SetAttribute("slice", []string{"a", "b"})
	span.SetAttribute("other", complex(1, 1))

	span.End()
}

func TestTracer_NoRenderer(t *testing.T) {
	tracer := telemetry.NewOTelTracer("test")
	ctx := context.Background()

	tracer.EmitPlan(ctx, []string{"task"}, map[string][]string{}, []string{})

	_, span := tracer.Start(ctx, "task")

	n, err := span.Write([]byte("log"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	span.End()
}

func TestBridge_NoRenderer(t *testing.T) {
	bridge := telemetry.NewBridge(nil)
	assert.NotNil(t, bridge)

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bridge))
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test")
	span.End()
}

func TestOTelTracer_Shutdown(t *testing.T) {
	tracer := telemetry.NewOTelTracer("test")
	ctx := context.Background()

	err := tracer.Shutdown(ctx)
	require.NoError(t, err)
}

func TestOTelSpan_RecordError(_ *testing.T) {
	tracer := telemetry.NewOTelTracer("test")
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test-error")
	testErr := errors.New("test error")
	span.RecordError(testErr)
	span.End()
}

func TestOTelTracer_LogBatching(t *testing.T) {
	mock := &mockRenderer{}
	tracer := telemetry.NewOTelTracer("test").WithRenderer(mock)
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test-span")

	for i := 0; i < 10; i++ {
		_, _ = span.Write([]byte("log"))
	}

	span.End()

	time.Sleep(100 * time.Millisecond)

	mock.mu.Lock()
	logCalls := mock.logCalls
	mock.mu.Unlock()
	assert.Positive(t, logCalls)
}
