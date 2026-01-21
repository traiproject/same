package telemetry_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.trai.ch/same/internal/adapters/telemetry"
)

// spyModel captures messages sent to the BubbleTea program.
type spyModel struct {
	msgs chan tea.Msg
}

func (m spyModel) Init() tea.Cmd { return nil }

func (m spyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Send message to channel without blocking if possible.
	select {
	case m.msgs <- msg:
	default:
		// Drop message if channel full
	}
	return m, nil
}

func (m spyModel) View() string { return "" }

// newSpyProgram creates a new tea.Program with a spy model and returns the message channel.
func newSpyProgram() (p *tea.Program, msgs chan tea.Msg) {
	msgs = make(chan tea.Msg, 100)
	model := spyModel{msgs: msgs}
	p = tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard))

	go func() {
		_, _ = p.Run()
	}()

	return p, msgs
}

func TestTUIBridge(t *testing.T) {
	p, msgs := newSpyProgram()
	defer p.Quit()

	// Initialize TUIBridge
	bridge := telemetry.NewTUIBridge(p)

	// Setup generic SDK tracer
	tp := trace.NewTracerProvider()
	tr := tp.Tracer("test-bridge")
	ctx := context.Background()

	t.Run("OnStart sends MsgTaskStart", func(t *testing.T) {
		_, span := tr.Start(ctx, "test-task")
		rwSpan, ok := span.(trace.ReadWriteSpan)
		require.True(t, ok, "span must implement ReadWriteSpan")

		bridge.OnStart(ctx, rwSpan)

		select {
		case msg := <-msgs:
			startMsg, ok := msg.(telemetry.MsgTaskStart)
			require.True(t, ok, "expected MsgTaskStart")
			assert.Equal(t, "test-task", startMsg.Name)
			assert.Equal(t, span.SpanContext().SpanID().String(), startMsg.SpanID)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for MsgTaskStart")
		}
		span.End()
	})

	t.Run("OnEnd sends MsgTaskComplete", func(t *testing.T) {
		_, span := tr.Start(ctx, "completed-task")
		rwSpan, ok := span.(trace.ReadWriteSpan)
		require.True(t, ok)

		span.End() // Transition to ReadOnly (mostly)

		bridge.OnEnd(rwSpan)

		select {
		case msg := <-msgs:
			endMsg, ok := msg.(telemetry.MsgTaskComplete)
			require.True(t, ok, "expected MsgTaskComplete")
			assert.Equal(t, span.SpanContext().SpanID().String(), endMsg.SpanID)
			require.NoError(t, endMsg.Err)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for MsgTaskComplete")
		}
	})

	t.Run("OnEnd sends MsgTaskComplete with error", func(t *testing.T) {
		_, span := tr.Start(ctx, "failed-task")
		span.RecordError(errors.New("something went wrong"))
		span.SetStatus(codes.Error, "failure description")

		rwSpan, ok := span.(trace.ReadWriteSpan)
		require.True(t, ok)
		span.End()

		bridge.OnEnd(rwSpan)

		select {
		case msg := <-msgs:
			endMsg, ok := msg.(telemetry.MsgTaskComplete)
			require.True(t, ok, "expected MsgTaskComplete")
			assert.Equal(t, span.SpanContext().SpanID().String(), endMsg.SpanID)
			require.Error(t, endMsg.Err)
			assert.Contains(t, endMsg.Err.Error(), "failure description")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for MsgTaskComplete")
		}
	})
}

func TestProvider(t *testing.T) {
	// Setup Valid TracerProvider for NewOTelTracer dependent tests
	// NewOTelTracer uses otel.Tracer(), which uses the global provider.
	// By default, it's a NoopProvider, which produces invalid SpanContexts.
	tp := trace.NewTracerProvider()
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(old)

	p, msgs := newSpyProgram()
	defer p.Quit()

	// 1. Test NewTracer
	tracer := telemetry.NewOTelTracer("test-provider")
	tracer.WithProgram(p)
	defer func() { _ = tracer.Shutdown(context.Background()) }()

	t.Run("Start creates valid span", func(t *testing.T) {
		ctx := context.Background()
		startCtx, span := tracer.Start(ctx, "provider-task")
		require.NotNil(t, span)
		require.NotNil(t, startCtx)

		s := oteltrace.SpanFromContext(startCtx)
		assert.True(t, s.SpanContext().IsValid())
		span.End()
	})

	t.Run("EmitPlan adds attributes and sends message", func(t *testing.T) {
		recorder := tracetest.NewSpanRecorder()
		tp := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))

		// Swap global provider so OTelTracer picks it up
		old := otel.GetTracerProvider()
		otel.SetTracerProvider(tp)
		defer otel.SetTracerProvider(old)

		tracerWithRecorder := telemetry.NewOTelTracer("test-recorder")
		tracerWithRecorder.WithProgram(p)
		defer func() { _ = tracerWithRecorder.Shutdown(context.Background()) }()

		ctx := context.Background()
		startCtx, span := tracerWithRecorder.Start(ctx, "planning-task")

		tasks := []string{"task-a", "task-b"}
		tracerWithRecorder.EmitPlan(startCtx, tasks)

		// 1. Verify Message sent to TUI
		select {
		case msg := <-msgs:
			initMsg, ok := msg.(telemetry.MsgInitTasks)
			if !ok {
				t.Fatalf("expected MsgInitTasks, got %T", msg)
			}
			assert.Equal(t, tasks, initMsg.Tasks)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for MsgInitTasks")
		}

		// 2. Verify Attributes in Trace
		span.End()

		spans := recorder.Ended()
		require.NotEmpty(t, spans)

		foundEvent := false
		for _, s := range spans {
			for _, e := range s.Events() {
				if e.Name == "plan_emitted" {
					for _, attr := range e.Attributes {
						if attr.Key == "tasks" {
							assert.Contains(t, attr.Value.Emit(), "task-a")
							assert.Contains(t, attr.Value.Emit(), "task-b")
							foundEvent = true
						}
					}
				}
			}
		}
		assert.True(t, foundEvent, "plan_emitted event should be recorded with tasks attribute")
	})
}
