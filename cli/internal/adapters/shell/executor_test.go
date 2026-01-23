package shell_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/shell"
	"go.trai.ch/same/internal/core/domain"
)

func TestExecutor_Execute_MultiLineOutput(t *testing.T) {
	executor := shell.NewExecutor()

	// Use a valid temporary directory for the working directory
	tmpDir := t.TempDir()

	task := &domain.Task{
		Name:       domain.NewInternedString("test-task"),
		Command:    []string{"sh", "-c", "echo line1; echo line2"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	var stdout bytes.Buffer
	err := executor.Execute(context.Background(), task, nil, &stdout, io.Discard)
	require.NoError(t, err)

	output := stdout.String()
	require.Contains(t, output, "line1")
	require.Contains(t, output, "line2")
}

func TestExecutor_Execute_FragmentedOutput(t *testing.T) {
	executor := shell.NewExecutor()
	tmpDir := t.TempDir()

	// Simulate fragmented write: "part1" then short sleep then "part2", then newline
	task := &domain.Task{
		Name:       domain.NewInternedString("test-fragmented"),
		Command:    []string{"sh", "-c", "printf part1; sleep 0.1; echo part2"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	var stdout bytes.Buffer
	err := executor.Execute(context.Background(), task, nil, &stdout, io.Discard)
	require.NoError(t, err)

	output := stdout.String()
	require.Contains(t, output, "part1")
	require.Contains(t, output, "part2")
}

func TestExecutor_Execute_EnvironmentVariables(t *testing.T) {
	executor := shell.NewExecutor()

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

	var stdout bytes.Buffer
	err := executor.Execute(context.Background(), task, nil, &stdout, io.Discard)
	require.NoError(t, err)

	output := stdout.String()
	require.Contains(t, output, "test-value-123")
}

func TestExecutor_Execute_InvalidCommand(t *testing.T) {
	executor := shell.NewExecutor()

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-invalid"),
		Command:    []string{"nonexistent-command-xyz123"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task, nil, io.Discard, io.Discard)
	if err == nil {
		t.Error("Execute() expected error for invalid command")
	}
}

func TestExecutor_Execute_CommandFailure(t *testing.T) {
	executor := shell.NewExecutor()

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-fail"),
		Command:    []string{"sh", "-c", "exit 42"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task, nil, io.Discard, io.Discard)
	if err == nil {
		t.Error("Execute() expected error for failed command")
	}

	// The error should wrap the exit error and include exit code
	if err != nil && !strings.Contains(err.Error(), "command failed") {
		t.Errorf("Execute() error should mention command failure: %v", err)
	}
}

func TestExecutor_Execute_EmptyCommand(t *testing.T) {
	executor := shell.NewExecutor()

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-empty"),
		Command:    []string{}, // Empty command
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	// Empty command should return nil without error
	err := executor.Execute(context.Background(), task, nil, io.Discard, io.Discard)
	if err != nil {
		t.Errorf("Execute() unexpected error for empty command: %v", err)
	}
}

func TestExecutor_Execute_AbsolutePath(t *testing.T) {
	executor := shell.NewExecutor()

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-absolute"),
		Command:    []string{"/bin/sh", "-c", "echo test"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	err := executor.Execute(context.Background(), task, nil, io.Discard, io.Discard)
	require.NoError(t, err)
}

func TestExecutor_Execute_WithNixEnv(t *testing.T) {
	executor := shell.NewExecutor()

	tmpDir := t.TempDir()
	task := &domain.Task{
		Name:       domain.NewInternedString("test-nix-env"),
		Command:    []string{"sh", "-c", "echo $NIX_VAR"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	nixEnv := []string{"NIX_VAR=nix-value"}
	var stdout bytes.Buffer
	err := executor.Execute(context.Background(), task, nixEnv, &stdout, io.Discard)
	require.NoError(t, err)

	output := stdout.String()
	require.Contains(t, output, "nix-value")
}

func TestExecutor_Execute_StreamsOutput(t *testing.T) {
	executor := shell.NewExecutor()
	tmpDir := t.TempDir()

	// Command outputting ANSI red color
	ansiRed := "\033[31m"
	ansiReset := "\033[0m"
	msg := "Hello Red World"
	task := &domain.Task{
		Name:       domain.NewInternedString("test-ansi"),
		Command:    []string{"sh", "-c", "printf '" + ansiRed + msg + ansiReset + "'"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	var stdout bytes.Buffer
	err := executor.Execute(context.Background(), task, nil, &stdout, io.Discard)
	require.NoError(t, err)

	output := stdout.String()
	// Verify ANSI codes are present
	if !strings.Contains(output, ansiRed) {
		t.Errorf("Expected output to contain ANSI red code, got: %q", output)
	}
	if !strings.Contains(output, msg) {
		t.Errorf("Expected output to contain message %q, got: %q", msg, output)
	}
}

type mockSpanWriter struct {
	data           []byte
	markExecCalled bool
}

func (m *mockSpanWriter) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func (m *mockSpanWriter) MarkExecStart() {
	m.markExecCalled = true
}

func TestExecutor_Execute_WithMarkExecStartSpan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()

	executor := shell.NewExecutor(mockLogger)
	tmpDir := t.TempDir()

	task := &domain.Task{
		Name:       domain.NewInternedString("test-mark-exec"),
		Command:    []string{"sh", "-c", "echo test"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	mockWriter := &mockSpanWriter{}
	err := executor.Execute(context.Background(), task, nil, mockWriter, io.Discard)
	require.NoError(t, err)

	assert.True(t, mockWriter.markExecCalled)
}

func TestExecutor_Execute_WithoutMarkExecStartSpan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()

	executor := shell.NewExecutor(mockLogger)
	tmpDir := t.TempDir()

	task := &domain.Task{
		Name:       domain.NewInternedString("test-no-mark-exec"),
		Command:    []string{"sh", "-c", "echo test"},
		WorkingDir: domain.NewInternedString(tmpDir),
	}

	var stdout bytes.Buffer
	err := executor.Execute(context.Background(), task, nil, &stdout, io.Discard)
	require.NoError(t, err)
}
