package progrock_test

import (
	"context"
	"testing"

	"go.trai.ch/bob/internal/adapters/telemetry/progrock"
	"go.trai.ch/bob/internal/core/domain"
)

func TestRecorder_Integration(t *testing.T) {
	// 1. Initialize the Recorder
	recorder := progrock.New()

	// 2. Start a task
	ctx := context.Background()
	_, vertex := recorder.Record(ctx, "Test Task")

	// 3. Write to Stdout
	if _, err := vertex.Stdout().Write([]byte("Standard Output\n")); err != nil {
		t.Errorf("failed to write to stdout: %v", err)
	}

	// 4. Log a debug message
	vertex.Log(domain.LogLevelDebug, "debug msg")

	// 5. Complete the vertex
	vertex.Complete(nil)

	// 6. Close the recorder
	if err := recorder.Close(); err != nil {
		t.Errorf("failed to close recorder: %v", err)
	}
}
