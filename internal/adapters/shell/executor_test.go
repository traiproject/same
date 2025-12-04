package shell_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/shell"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestExecutor_Execute_MultiLineOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect Info to be called twice, once for each line
	// We use gomock.InOrder to ensure order, though not strictly required by prompt, it's good practice.
	// However, the prompt just says "exactly twice".
	mockLogger.EXPECT().Info("line1").Times(1)
	mockLogger.EXPECT().Info("line2").Times(1)

	executor := shell.NewExecutor(mockLogger)

	// Use a valid temporary directory for the working directory
	tmpDir := t.TempDir()

	task := &domain.Task{
		Name:       domain.NewInternedString("test-task"),
		Command:    []string{"sh", "-c", "echo line1; echo line2"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task)
	require.NoError(t, err)
}

func TestExecutor_Execute_EnvironmentVariables(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)

	// Expect the environment variable value to be logged
	mockLogger.EXPECT().Info("test-value-123").Times(1)

	executor := shell.NewExecutor(mockLogger)

	// Use a valid temporary directory for the working directory
	tmpDir := t.TempDir()

	task := &domain.Task{
		Name:    domain.NewInternedString("test-env-task"),
		Command: []string{"sh", "-c", "echo $MY_TEST_VAR"},
		Environment: map[string]string{
			"MY_TEST_VAR": "test-value-123",
		},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task)
	require.NoError(t, err)
}
