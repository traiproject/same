package shell_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/shell"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestResolveEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sysEnv   []string
		nixEnv   []string
		taskEnv  map[string]string
		expected []string
	}{
		{
			name:   "Basic Precedence: Task > Nix > System",
			sysEnv: []string{"PATH=/bin", "USER=me", "FORBIDDEN=secret"},
			nixEnv: []string{"PATH=/nix/bin", "NIX_VAR=foo"},
			taskEnv: map[string]string{
				"PATH":     "/custom/bin",
				"TASK_VAR": "bar",
			},
			expected: []string{
				"PATH=/custom/bin", // Task override
				"USER=me",          // System allow-listed
				"NIX_VAR=foo",      // Nix
				"TASK_VAR=bar",     // Task
				// FORBIDDEN should be missing
			},
		},
		{
			name:    "Path Merging (Nix Prepend)",
			sysEnv:  []string{"PATH=/system/bin"},
			nixEnv:  []string{"PATH=/nix/bin"},
			taskEnv: map[string]string{
				// No PATH override in task
			},
			expected: []string{
				"PATH=/nix/bin" + string(os.PathListSeparator) + "/system/bin",
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := shell.ResolveEnvironment(tt.sysEnv, tt.nixEnv, tt.taskEnv)

			// Convert result to map for easier assertion
			resultMap := make(map[string]string)
			for _, entry := range result {
				parts := strings.SplitN(entry, "=", 2)
				resultMap[parts[0]] = parts[1]
			}

			for _, exp := range tt.expected {
				parts := strings.SplitN(exp, "=", 2)
				key := parts[0]
				val := parts[1]
				assert.Equal(t, val, resultMap[key], "Mismatch for key %s", key)
			}

			// Explicitly check for forbidden vars if mentioned in test
			if strings.Contains(strings.Join(tt.sysEnv, ","), "FORBIDDEN") {
				_, exists := resultMap["FORBIDDEN"]
				assert.False(t, exists, "FORBIDDEN var should not exist")
			}
		})
	}
}

func TestExecutor_Execute(t *testing.T) {
	// Note: We use synctest-like logic where possible, but for OS exec we just rely on standard testing
	// Executor uses pty, so we should test basic command execution.

	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Allow any log info calls (pty implementation logs stdout/stderr to logger)
	mockLogger.EXPECT().Info(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any()).AnyTimes()

	exec := shell.NewExecutor(mockLogger)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		task := &domain.Task{
			Command:    []string{"echo", "hello"},
			WorkingDir: domain.NewInternedString(""), // Initialize to avoid panic
		}

		err := exec.Execute(ctx, task, nil, &stdout, &stderr)
		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "hello")
	})

	t.Run("Failure", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		// sh -c 'exit 1'
		task := &domain.Task{
			Command:    []string{"sh", "-c", "exit 1"},
			WorkingDir: domain.NewInternedString(""), // Initialize to avoid panic
		}

		err := exec.Execute(ctx, task, nil, &stdout, &stderr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command failed")
	})

	t.Run("WorkingDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Evaluate symlinks to get the real path, as execution might resolve it
		realTmpDir, err := filepath.EvalSymlinks(tmpDir)
		require.NoError(t, err)

		var stdout, stderr bytes.Buffer
		task := &domain.Task{
			Command:    []string{"pwd"},
			WorkingDir: domain.NewInternedString(realTmpDir),
		}

		err = exec.Execute(ctx, task, nil, &stdout, &stderr)
		require.NoError(t, err)

		// Output of pwd should contain the temp dir
		// Note: PTYs might add \r\n
		output := strings.TrimSpace(stdout.String())
		// Handle PTY behavior where it might output \r\n
		output = strings.ReplaceAll(output, "\r", "")

		// On macOS /var is a link to /private/var, so strict equality might fail unless we eval symlinks
		// We did that above.
		assert.Equal(t, realTmpDir, output)
	})
}
