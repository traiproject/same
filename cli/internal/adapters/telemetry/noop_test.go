package telemetry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/telemetry"
)

func TestNoOpTracer_Start(t *testing.T) {
	t.Parallel()

	tracer := telemetry.NewNoOpTracer()
	ctx := context.Background()

	newCtx, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, newCtx)
	assert.NotNil(t, span)

	span.End()
}

func TestNoOpTracer_EmitPlan(t *testing.T) {
	t.Parallel()

	tracer := telemetry.NewNoOpTracer()
	ctx := context.Background()

	tracer.EmitPlan(ctx, []string{"task1", "task2"}, map[string][]string{"task1": {"task2"}}, []string{"task1"})
}

func TestNoOpSpan_End(t *testing.T) {
	t.Parallel()

	tracer := telemetry.NewNoOpTracer()
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test")
	span.End()
}

func TestNoOpSpan_RecordError(t *testing.T) {
	t.Parallel()

	tracer := telemetry.NewNoOpTracer()
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test")
	span.RecordError(errors.New("test error"))
	span.End()
}

func TestNoOpSpan_SetAttribute(t *testing.T) {
	t.Parallel()

	tracer := telemetry.NewNoOpTracer()
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test")
	span.SetAttribute("key", "value")
	span.SetAttribute("int", 123)
	span.SetAttribute("bool", true)
	span.End()
}

func TestNoOpSpan_Write(t *testing.T) {
	t.Parallel()

	tracer := telemetry.NewNoOpTracer()
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test")
	n, err := span.Write([]byte("test log data"))
	require.NoError(t, err)
	assert.Equal(t, 13, n)
	span.End()
}

func TestNoOpSpan_MarkExecStart(t *testing.T) {
	t.Parallel()

	tracer := telemetry.NewNoOpTracer()
	ctx := context.Background()

	_, span := tracer.Start(ctx, "test")
	span.MarkExecStart()
	span.End()
}
