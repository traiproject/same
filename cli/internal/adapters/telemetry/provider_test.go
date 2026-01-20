package telemetry_test

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

	// We can use a dummy program model that records messages.
	model := &testModel{
		msgs: make(chan tea.Msg, 10),
	}
	prog := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(nil)) // Headless

	tracer.WithProgram(prog)

	ctx := context.Background()
	tracer.EmitPlan(ctx, []string{"task1", "task2"})

	// Wait for span
	_ = tp.ForceFlush(ctx)
	spans := sr.Ended()
	// EmitPlan uses trace.SpanFromContext(ctx) which is empty here,
	// so no attributes added to a span unless we create one.
	assert.Empty(t, spans)

	// Create a span context
	ctx, span := tp.Tracer("test").Start(ctx, "root")
	tracer.EmitPlan(ctx, []string{"task1", "task2"})
	span.End()

	_ = tp.ForceFlush(ctx)
	spans = sr.Ended()
	require.Len(t, spans, 1)

	// Check events
	events := spans[0].Events()
	require.Len(t, events, 1)
	assert.Equal(t, "plan_emitted", events[0].Name)
}

func TestOTelTracer_WithProgram_And_Start(t *testing.T) {
	// Custom tracer to peek at internals if needed, or just use public API.
	tracer := telemetry.NewOTelTracer("test-tracer")
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	// Let's just verify state.
	prog := tea.NewProgram(nil)
	tracer.WithProgram(prog)

	ctx, span := tracer.Start(context.Background(), "test-span")
	otelSpan, ok := span.(*telemetry.OTelSpan)
	require.True(t, ok)

	// If program is set, batcher should be initialized
	assert.NotNil(t, otelSpan.Batcher())

	span.End()
	_ = ctx // usage to avoid unused check if needed, but span.End() doesn't need ctx
	// Batcher should be closed/nil (internally).
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
	span.SetAttribute("unknown", struct{}{}) // Should fall to default case.

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

	// Case 1: No program (no batcher).
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

// testModel is a dummy tea.Model to capture messages.
type testModel struct {
	msgs chan tea.Msg
}

func (m *testModel) Init() tea.Cmd { return nil }
func (m *testModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	select {
	case m.msgs <- msg:
	default:
	}
	return m, nil
}
func (m *testModel) View() string { return "" }
