package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.trai.ch/same/internal/adapters/telemetry"
)

func setupMonitor() (*tracetest.SpanRecorder, *trace.TracerProvider) {
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	otel.SetTracerProvider(tp)
	return sr, tp
}

func TestOTelTracer_EmitPlan(t *testing.T) {
	sr, tp := setupMonitor()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := telemetry.NewOTelTracer("test-tracer")
	mock := &mockRenderer{}
	tracer.WithRenderer(mock)

	ctx := context.Background()
	tracer.EmitPlan(ctx, []string{"task1", "task2"}, map[string][]string{}, []string{})

	_ = tp.ForceFlush(ctx)
	spans := sr.Ended()
	assert.Empty(t, spans)

	assert.Equal(t, 1, mock.planCalls)

	ctx, span := tp.Tracer("test").Start(ctx, "root")
	tracer.EmitPlan(ctx, []string{"task1", "task2"}, map[string][]string{}, []string{})
	span.End()

	_ = tp.ForceFlush(ctx)
	spans = sr.Ended()
	require.Len(t, spans, 1)

	events := spans[0].Events()
	require.Len(t, events, 1)
	assert.Equal(t, "plan_emitted", events[0].Name)
}

func TestOTelTracer_WithRenderer_And_Start(t *testing.T) {
	tracer := telemetry.NewOTelTracer("test-tracer")
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	mock := &mockRenderer{}
	tracer.WithRenderer(mock)

	ctx, span := tracer.Start(context.Background(), "test-span")
	_, err := span.Write([]byte("test log"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	mock.mu.Lock()
	logCount := mock.logCalls
	mock.mu.Unlock()

	assert.Positive(t, logCount)

	span.End()
	_ = ctx
}

func TestOTelSpan_SetAttribute(t *testing.T) {
	sr, tp := setupMonitor()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := telemetry.NewOTelTracer("test-tracer")
	ctx, span := tracer.Start(context.Background(), "attr-test")

	span.SetAttribute("str", "val")
	span.SetAttribute("int", 123)
	span.SetAttribute("int64", int64(456))
	span.SetAttribute("float", 3.14)
	span.SetAttribute("bool", true)
	span.SetAttribute("slice", []string{"a", "b"})
	span.SetAttribute("unknown", struct{}{})

	span.End()

	_ = tp.ForceFlush(ctx)
	spans := sr.Ended()
	require.Len(t, spans, 1)

	attrs := spans[0].Attributes()
	attrMap := make(map[string]any)
	for _, a := range attrs {
		switch a.Value.Type() {
		case attribute.STRING:
			attrMap[string(a.Key)] = a.Value.AsString()
		case attribute.INT64:
			attrMap[string(a.Key)] = a.Value.AsInt64()
		case attribute.FLOAT64:
			attrMap[string(a.Key)] = a.Value.AsFloat64()
		case attribute.BOOL:
			attrMap[string(a.Key)] = a.Value.AsBool()
		case attribute.STRINGSLICE:
			attrMap[string(a.Key)] = a.Value.AsStringSlice()
		}
	}

	assert.Equal(t, "val", attrMap["str"])
	assert.Equal(t, int64(123), attrMap["int"])
	assert.Equal(t, int64(456), attrMap["int64"])
	assert.InEpsilon(t, 3.14, attrMap["float"], 0.001)
	assert.Equal(t, true, attrMap["bool"])
	assert.Equal(t, []string{"a", "b"}, attrMap["slice"])
	assert.Equal(t, "{}", attrMap["unknown"])
}

func TestOTelSpan_Write(t *testing.T) {
	sr, tp := setupMonitor()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := telemetry.NewOTelTracer("test-tracer")

	ctx, span := tracer.Start(context.Background(), "log-test-no-prog")
	n, err := span.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	span.End()

	_ = tp.ForceFlush(ctx)
	spans := sr.Ended()
	require.Len(t, spans, 1)

	events := spans[0].Events()
	require.Len(t, events, 1)
	assert.Equal(t, "log", events[0].Name)
	assert.Equal(t, "hello", events[0].Attributes[0].Value.AsString())
}

func TestOTelTracer_Shutdown_AfterStart(t *testing.T) {
	tracer := telemetry.NewOTelTracer("test-shutdown")

	ctx := context.Background()
	err := tracer.Shutdown(ctx)
	require.NoError(t, err)

	_, span := tracer.Start(ctx, "after-shutdown")
	span.End()
}

func TestOTelTracer_Start_WithoutRenderer(t *testing.T) {
	tracer := telemetry.NewOTelTracer("test-no-renderer")
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	ctx, span := tracer.Start(context.Background(), "no-renderer-span")
	otelSpan, ok := span.(*telemetry.OTelSpan)
	require.True(t, ok)

	assert.Nil(t, otelSpan.Batcher())
	span.End()
	_ = ctx
}

func TestOTelTracer_Start_WithRenderer(t *testing.T) {
	tracer := telemetry.NewOTelTracer("test-with-renderer")
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	mock := &mockRenderer{}
	tracer.WithRenderer(mock)

	ctx, span := tracer.Start(context.Background(), "with-renderer-span")
	otelSpan, ok := span.(*telemetry.OTelSpan)
	require.True(t, ok)

	assert.NotNil(t, otelSpan.Batcher())
	span.End()
	_ = ctx
}

func TestOTelSpan_Write_WithBatcher(t *testing.T) {
	tracer := telemetry.NewOTelTracer("test-write-batcher")
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	mock := &mockRenderer{}
	tracer.WithRenderer(mock)

	ctx, span := tracer.Start(context.Background(), "batcher-write-test")

	n, err := span.Write([]byte("test data"))
	require.NoError(t, err)
	assert.Equal(t, 9, n)

	span.End()
	_ = ctx
}

func TestOTelSpan_RecordError_WithStatus(t *testing.T) {
	sr, tp := setupMonitor()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := telemetry.NewOTelTracer("test-error")
	ctx, span := tracer.Start(context.Background(), "error-test")

	testErr := assert.AnError
	span.RecordError(testErr)
	span.End()

	_ = tp.ForceFlush(ctx)
	spans := sr.Ended()
	require.Len(t, spans, 1)

	assert.Contains(t, spans[0].Status().Description, testErr.Error())
}

func TestOTelSpan_MarkExecStart_WithoutEventSender(_ *testing.T) {
	tracer := telemetry.NewOTelTracer("test-mark-exec-no-sender")
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	ctx, span := tracer.Start(context.Background(), "no-event-sender")

	span.MarkExecStart()

	span.End()
	_ = ctx
}

func TestOTelSpan_End_WithBatcher(_ *testing.T) {
	tracer := telemetry.NewOTelTracer("test-end-batcher")
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	mock := &mockRenderer{}
	tracer.WithRenderer(mock)

	ctx, span := tracer.Start(context.Background(), "end-batcher-test")

	_, _ = span.Write([]byte("data before end"))

	span.End()

	_ = ctx
}
