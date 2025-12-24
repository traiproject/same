package shell_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestExecutor_Execute_WithVertex(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	// Logger shouldn't be used when Vertex is present
	mockLogger.EXPECT().Info(gomock.Any()).Times(0)
	mockLogger.EXPECT().Error(gomock.Any()).Times(0)

	mockVertex := mocks.NewMockVertex(ctrl)

	// Buffers to capture output
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	mockVertex.EXPECT().Stdout().Return(&stdoutBuf).AnyTimes()
	mockVertex.EXPECT().Stderr().Return(&stderrBuf).AnyTimes()

	executor := shell.NewExecutor(mockLogger)
	tmpDir := t.TempDir()

	task := &domain.Task{
		Name:       domain.NewInternedString("test-vertex"),
		Command:    []string{"sh", "-c", "echo hello to stdout; echo hello to stderr >&2"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	// Inject Vertex into context
	ctx := ports.ContextWithVertex(context.Background(), mockVertex)

	err := executor.Execute(ctx, task, nil)
	require.NoError(t, err)

	require.Contains(t, stdoutBuf.String(), "hello to stdout")
	require.Contains(t, stderrBuf.String(), "hello to stderr")
}
