package shell_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestExecutor_Execute_HermeticBinaryOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	// Expect implicit failure log if it fails, or success log if it succeeds
	// If it succeeds, the script prints "success", which executor logs as Info
	mockLogger.EXPECT().Info("success").Times(1)

	executor := shell.NewExecutor(mockLogger)

	// Create a temp directory to act as our "hermetic" bin path
	hermeticDir := t.TempDir()

	// Create a dummy executable script "my-hermetic-tool"
	cmdName := "my-hermetic-tool"
	cmdPath := filepath.Join(hermeticDir, cmdName)
	content := "#!/bin/sh\necho success\n"
	//nolint:gosec // Test requires executable file
	err := os.WriteFile(cmdPath, []byte(content), 0o700)
	require.NoError(t, err)

	task := &domain.Task{
		Name:    domain.NewInternedString("test-hermetic"),
		Command: []string{cmdName},
		// WorkingDir doesn't matter for specific path lookup, but required by task
		WorkingDir: domain.NewInternedString(hermeticDir),
	}

	// Provide the hermetic PATH in env
	nixEnv := []string{"PATH=" + hermeticDir}

	err = executor.Execute(context.Background(), task, nixEnv, io.Discard, io.Discard)
	require.NoError(t, err)
}
