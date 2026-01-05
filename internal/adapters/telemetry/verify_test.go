package telemetry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/telemetry"
	"go.trai.ch/bob/internal/core/ports"
)

func TestInterfaceSatisfaction(_ *testing.T) {
	var _ ports.Tracer = (*telemetry.OTelTracer)(nil)
	var _ ports.Span = (*telemetry.OTelSpan)(nil)
	var _ ports.Tracer = (*telemetry.NoOpTracer)(nil)
	var _ ports.Span = (*telemetry.NoOpSpan)(nil)
}

func TestOTelTracer_Start(t *testing.T) {
	// This test assumes OTel SDK is available.
	// We just check for no panic during instantiation and Start.
	tracer := telemetry.NewOTelTracer("test-tracer")
	assert.NotNil(t, tracer)

	ctx := context.Background()
	_, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, span)

	span.SetAttribute("key", "value")
	n, err := span.Write([]byte("test log"))
	require.NoError(t, err)
	assert.Equal(t, 8, n)

	span.End()
}

func TestNoOpTracer_Start(t *testing.T) {
	tracer := telemetry.NewNoOpTracer()
	assert.NotNil(t, tracer)

	ctx := context.Background()
	_, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, span)

	span.SetAttribute("key", "value")
	n, err := span.Write([]byte("test log"))
	require.NoError(t, err)
	assert.Equal(t, 8, n)

	span.End()
}
