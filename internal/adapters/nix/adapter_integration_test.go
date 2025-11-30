//go:build integration

package nix

import (
	"context"
	"fmt"
	"os/exec"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_Integration(t *testing.T) {
	// Check if nix is available
	if _, err := os.Stat("/nix"); os.IsNotExist(err) {
		t.Skip("Nix is not installed, skipping integration test")
	}

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nix_cache.json")
	adapter := New(cachePath)

	// Use a simple package that should be quick to evaluate
	// "hello" is a classic small package
	deps := []string{"hello"}

	// Check nix version
	out, verErr := exec.Command("nix", "--version").CombinedOutput()
	fmt.Printf("nix --version: %s (err: %v)\n", out, verErr)

	vars, err := adapter.Resolve(context.Background(), deps)
	if err != nil {
		fmt.Printf("Resolve failed: %+v\n", err)
		// Check if nix is in PATH
		path, pathErr := exec.LookPath("nix")
		fmt.Printf("nix path: %s (err: %v)\n", path, pathErr)
	}
	require.NoError(t, err)

	// Verify we got some expected variables
	assert.Contains(t, vars, "PATH")
	assert.Contains(t, vars["PATH"], "hello")

	// Verify cache was created
	_, err = os.Stat(cachePath)
	assert.NoError(t, err)

	// Verify cache hit on second run
	// We can't easily verify it *was* a hit without logs or metrics,
	// but we can verify it still works and is fast.
	vars2, err := adapter.Resolve(context.Background(), deps)
	require.NoError(t, err)
	assert.Equal(t, vars, vars2)
}
