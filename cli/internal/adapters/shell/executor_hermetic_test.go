package shell_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/shell"
	"go.trai.ch/same/internal/core/domain"
)

func TestExecutor_Execute_HermeticBinaryOnly(t *testing.T) {
	executor := shell.NewExecutor()

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

	var stdout bytes.Buffer
	err = executor.Execute(context.Background(), task, nixEnv, &stdout, &stdout)
	require.NoError(t, err)

	output := stdout.String()
	require.Contains(t, output, "success")
}
