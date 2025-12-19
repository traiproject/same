package shell_test

import (
	"context"
	"strings"
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

	err := executor.Execute(context.Background(), task, nil)
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

	err := executor.Execute(context.Background(), task, nil)
	require.NoError(t, err)
}

func TestExecutor_Execute_InvalidCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)

	// For non-existent command, expect error to be logged
	mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()

	executor := shell.NewExecutor(mockLogger)

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-invalid"),
		Command:    []string{"nonexistent-command-xyz123"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task, nil)
	if err == nil {
		t.Error("Execute() expected error for invalid command")
	}
}

func TestExecutor_Execute_CommandFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)

	// Command will output to stderr, so expect Error calls
	mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()

	executor := shell.NewExecutor(mockLogger)

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-fail"),
		Command:    []string{"sh", "-c", "exit 42"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task, nil)
	if err == nil {
		t.Error("Execute() expected error for failed command")
	}

	// The error should wrap the exit error and include exit code
	if err != nil && !strings.Contains(err.Error(), "command failed") {
		t.Errorf("Execute() error should mention command failure: %v", err)
	}
}

func TestExecutor_Execute_EmptyCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	executor := shell.NewExecutor(mockLogger)

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-empty"),
		Command:    []string{}, // Empty command
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	// Empty command should return nil without error
	err := executor.Execute(context.Background(), task, nil)
	if err != nil {
		t.Errorf("Execute() unexpected error for empty command: %v", err)
	}
}

func TestExecutor_Execute_AbsolutePath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()

	executor := shell.NewExecutor(mockLogger)

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-absolute"),
		Command:    []string{"/bin/sh", "-c", "echo test"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task, nil)
	require.NoError(t, err)
}

func TestExecutor_Execute_WithNixEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info("nix-value").Times(1)

	executor := shell.NewExecutor(mockLogger)

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-nix-env"),
		Command:    []string{"sh", "-c", "echo $NIX_VAR"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	nixEnv := []string{"NIX_VAR=nix-value"}
	err := executor.Execute(context.Background(), task, nixEnv)
	require.NoError(t, err)
}
