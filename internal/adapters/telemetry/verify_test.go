package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.trai.ch/bob/internal/core/ports"
)

func TestInterfaceSatisfaction(t *testing.T) {
	var _ ports.Tracer = (*OTelTracer)(nil)
	var _ ports.Span = (*OTelSpan)(nil)
	var _ ports.Tracer = (*NoOpTracer)(nil)
	var _ ports.Span = (*NoOpSpan)(nil)
}

func TestOTelTracer_Start(t *testing.T) {
	// This test assumes OTel SDK is available.
	// We just check for no panic during instantiation and Start.
	tracer := NewOTelTracer("test-tracer")
	assert.NotNil(t, tracer)

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, span)
	
	span.SetAttribute("key", "value")
	n, err := span.Write([]byte("test log"))
	assert.NoError(t, err)
	assert.Equal(t, 8, n)

	span.End()
}

func TestNoOpTracer_Start(t *testing.T) {
	tracer := NewNoOpTracer()
	assert.NotNil(t, tracer)

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, span)

	span.SetAttribute("key", "value")
	n, err := span.Write([]byte("test log"))
	assert.NoError(t, err)
	assert.Equal(t, 8, n)

	span.End()
}
