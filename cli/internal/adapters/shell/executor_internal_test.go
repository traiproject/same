package shell

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		sysEnv   []string
		nixEnv   []string
		taskEnv  map[string]string
		expected []string
	}{
		{
			name:     "System Only (Allowed)",
			sysEnv:   []string{"USER=test", "PATH=/bin", "HOME=/home/test"},
			nixEnv:   nil,
			taskEnv:  nil,
			expected: []string{"USER=test", "PATH=/bin", "HOME=/home/test"},
		},
		{
			name:     "System Only (Filtered)",
			sysEnv:   []string{"USER=test", "SSH_AUTH_SOCK=/tmp/ssh", "SECRET=key"},
			nixEnv:   nil,
			taskEnv:  nil,
			expected: []string{"USER=test"},
		},
		{
			name:     "System + Nix (No PATH)",
			sysEnv:   []string{"USER=test", "PATH=/bin"},
			nixEnv:   []string{"NIX_CC=gcc"},
			taskEnv:  nil,
			expected: []string{"USER=test", "PATH=/bin", "NIX_CC=gcc"},
		},
		{
			name:     "System + Nix (Prepend PATH)",
			sysEnv:   []string{"USER=test", "PATH=/bin"},
			nixEnv:   []string{"PATH=/nix/bin", "NIX_CC=gcc"},
			taskEnv:  nil,
			expected: []string{"USER=test", "PATH=/nix/bin" + string(os.PathListSeparator) + "/bin", "NIX_CC=gcc"},
		},
		{
			name:     "System + Nix + Task (Override)",
			sysEnv:   []string{"USER=test", "PATH=/bin"},
			nixEnv:   []string{"PATH=/nix/bin"},
			taskEnv:  map[string]string{"USER": "same", "FOO": "bar"},
			expected: []string{"USER=same", "PATH=/nix/bin" + string(os.PathListSeparator) + "/bin", "FOO=bar"},
		},
		{
			name:     "System + Nix + Task (Task PATH override)",
			sysEnv:   []string{"USER=test", "PATH=/bin"},
			nixEnv:   []string{"PATH=/nix/bin"},
			taskEnv:  map[string]string{"PATH": "/custom/bin"},
			expected: []string{"USER=test", "PATH=/custom/bin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEnvironment(tt.sysEnv, tt.nixEnv, tt.taskEnv)

			// Sort for deterministic comparison
			sort.Strings(got)
			sort.Strings(tt.expected)

			assert.Equal(t, tt.expected, got)
		})
	}
}

// Ensure it handles empty System Env correctly (adds Nix PATH if Sys PATH missing).
func TestResolveEnvironment_EmptySystem(t *testing.T) {
	sysEnv := []string{}
	nixEnv := []string{"PATH=/nix/bin"}
	taskEnv := map[string]string{}

	got := resolveEnvironment(sysEnv, nixEnv, taskEnv)
	assert.Contains(t, got, "PATH=/nix/bin")
	// Should not have separator if sys is empty/missing
	// Wait, code: if sysPath, exists := envMap["PATH"]; exists && sysPath != ""
	// If empty, map doesn't have it.
	// So just /nix/bin
}

func TestLookPath_EmptyPATH(t *testing.T) {
	// Environment with no PATH variable
	env := []string{"USER=test"}
	_, err := lookPath("echo", env)
	if err == nil {
		t.Error("lookPath() expected error when PATH is not in environment")
	}
}

func TestLookPath_ExecutableNotFound(t *testing.T) {
	env := []string{"PATH=/nonexistent/dir"}
	_, err := lookPath("nonexistent-command", env)
	if err == nil {
		t.Error("lookPath() expected error when executable not found")
	}
}

func TestLookPath_EmptyDirectory(t *testing.T) {
	// PATH with empty directory should default to "."
	tmpDir := t.TempDir()

	env := []string{"PATH=:" + tmpDir} // Empty directory before colon
	_, err := lookPath("nonexistent", env)
	if err == nil {
		t.Error("lookPath() expected error when executable not found even with empty dir")
	}
}

func TestFindExecutable_NonExistent(t *testing.T) {
	err := findExecutable("/nonexistent/file")
	if err == nil {
		t.Error("findExecutable() expected error for non-existent file")
	}
}

func TestFindExecutable_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	err := findExecutable(tmpDir)
	if err == nil {
		t.Error("findExecutable() expected error for directory")
	}
}

func TestPtyProcess_Resize_BoundsChecking(t *testing.T) {
	proc := &ptyProcess{}

	tests := []struct {
		name string
		rows int
		cols int
	}{
		{"negative rows", -1, 80},
		{"negative cols", 24, -1},
		{"rows too large", 100000, 80},
		{"cols too large", 24, 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := proc.Resize(tt.rows, tt.cols)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "out of bounds")
		})
	}
}
