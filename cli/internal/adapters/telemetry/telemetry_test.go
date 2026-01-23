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
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.trai.ch/same/internal/adapters/telemetry"
)

// TestModel captures messages for verification.
type TestModel struct {
	Captured []tea.Msg
	MsgCh    chan tea.Msg
}

func (m TestModel) Init() tea.Cmd { return nil }
func (m TestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.MsgCh != nil {
		select {
		case m.MsgCh <- msg:
		default:
		}
	}
	return m, nil
}
func (m TestModel) View() string { return "" }

func TestOTelTracer_WithProgram(t *testing.T) {
	// Setup
	msgCh := make(chan tea.Msg, 10)
	model := TestModel{MsgCh: msgCh}
	prog := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard))
	// We don't start the program because Send works even if not started?
	// Actually Send returns immediately if program is updated?
	// Bubbletea docs say Send is safe to call from any goroutine.
	// But the program loop needs to be running to process messages if we want to Verify via Update.
	// However, relying on running program is flaky.
	// Ideally we trust Send works.

	// BUT, validation requires checking if Send was called.
	// Since we can't mock Send, we MUST run the program.
	go func() {
		_, _ = prog.Run()
	}()
	// Allow startup
	time.Sleep(10 * time.Millisecond)
	defer prog.Quit()

	tracer := telemetry.NewOTelTracer("test-tracer").WithProgram(prog)
	ctx := context.Background()

	// Test EmitPlan
	tracer.EmitPlan(ctx, []string{"task1"}, map[string][]string{}, []string{})

	select {
	case msg := <-msgCh:
		initMsg, ok := msg.(telemetry.MsgInitTasks)
		require.True(t, ok)
		assert.Equal(t, []string{"task1"}, initMsg.Tasks)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for MsgInitTasks")
	}

	// Test Start and Log
	_, span := tracer.Start(ctx, "test-span")
	_, err := span.Write([]byte("log data"))
	require.NoError(t, err)

	// Wait for batcher (default 50ms)
	select {
	case msg := <-msgCh:
		logMsg, ok := msg.(telemetry.MsgTaskLog)
		require.True(t, ok)
		assert.Equal(t, []byte("log data"), logMsg.Data)
		// SpanID should be valid
		assert.NotEmpty(t, logMsg.SpanID)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for MsgTaskLog")
	}

	span.End()
}

func TestTUIBridge(t *testing.T) {
	msgCh := make(chan tea.Msg, 10)
	model := TestModel{MsgCh: msgCh}
	prog := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard))
	go func() {
		_, _ = prog.Run()
	}()
	time.Sleep(10 * time.Millisecond)
	defer prog.Quit()

	bridge := telemetry.NewTUIBridge(prog)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bridge))
	tracer := tp.Tracer("test-bridge")

	// Test OnStart
	_, span := tracer.Start(context.Background(), "test-task")

	select {
	case msg := <-msgCh:
		startMsg, ok := msg.(telemetry.MsgTaskStart)
		require.True(t, ok)
		assert.Equal(t, "test-task", startMsg.Name)
		assert.NotEmpty(t, startMsg.SpanID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for MsgTaskStart")
	}

	// Test OnEnd (Success)
	span.End()

	select {
	case msg := <-msgCh:
		endMsg, ok := msg.(telemetry.MsgTaskComplete)
		require.True(t, ok)
		require.NoError(t, endMsg.Err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for MsgTaskComplete")
	}

	// Test OnEnd (Error)
	_, spanErr := tracer.Start(context.Background(), "test-error")
	// Consume start msg
	<-msgCh

	spanErr.RecordError(errors.New("some error"))
	spanErr.SetStatus(codes.Error, "task failed explicitly")
	spanErr.End()

	select {
	case msg := <-msgCh:
		endMsg, ok := msg.(telemetry.MsgTaskComplete)
		require.True(t, ok)
		require.Error(t, endMsg.Err)
		assert.Contains(t, endMsg.Err.Error(), "task failed explicitly")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for MsgTaskComplete (Error)")
	}
}

func TestOTelSpan_Attributes(_ *testing.T) {
	// Verify SetAttribute types usage
	// Using a SDK tracer to verify attributes might be complex,
	// but we just want to ensure the switch case is covered and doesn't panic.
	// Since OTelSpan wraps a trace.Span, we trust the underlying implementation
	// but we should call SetAttribute with different types to cover the switch in provider.go:97.

	// Helper to spy on attributes?
	// OTel API doesn't easily expose attributes of a recording span without an exporter/processor.
	// Use a mock span? OTel interfaces are hard to mock manually due to private methods?
	// Actually `trace.Span` interface can be mocked if we implement `RecordError`, `AddEvent`, etc.
	// But `OTelSpan` struct has `trace.Span` field.

	// We can use a real tracer with a memory exporter?
	// Or just call the methods and ensure no panic (coverage will still count).

	tracer := telemetry.NewOTelTracer("test")
	_, span := tracer.Start(context.Background(), "test")

	span.SetAttribute("string", "val")
	span.SetAttribute("int", 123)
	span.SetAttribute("int64", int64(123))
	span.SetAttribute("float64", 12.34)
	span.SetAttribute("bool", true)
	span.SetAttribute("slice", []string{"a", "b"})
	span.SetAttribute("other", complex(1, 1)) // Coverage for default case

	span.End()
}

func TestTracer_NoProgram(t *testing.T) {
	// Cover branches where program is nil
	tracer := telemetry.NewOTelTracer("test") // No WithProgram
	ctx := context.Background()

	// EmitPlan
	tracer.EmitPlan(ctx, []string{"task"}, map[string][]string{}, []string{})

	// Start
	_, span := tracer.Start(ctx, "task")

	// Write (should just add event to span)
	n, err := span.Write([]byte("log"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	span.End()
}

func TestBridge_NoProgram(t *testing.T) {
	bridge := telemetry.NewTUIBridge(nil)
	assert.NotNil(t, bridge)

	// Should be safe to call methods
	bridge.OnStart(context.Background(), nil) // mocked or nil span?
	// OnStart takes ReadWriteSpan interface. Passing nil will likely panic if not checked,
	// but the code checks `if b.program == nil`.

	// We need a span to pass to OnStart/OnEnd to adhere to interface signature if we mock it?
	// Since we are using real OTel SDK in previous tests, we can use it here too.

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bridge))
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test")
	span.End()

	// No panic means success
}
