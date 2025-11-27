package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/fs"
)

func TestVerifier_VerifyOutputs(t *testing.T) {
	tmpDir := t.TempDir()
	verifier := fs.NewVerifier()

	// Case 1: All outputs exist
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "out1.txt"), []byte("content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "out2.txt"), []byte("content"), 0o600))

	exists, err := verifier.VerifyOutputs(tmpDir, []string{"out1.txt", "out2.txt"})
	require.NoError(t, err)
	assert.True(t, exists)

	// Case 2: One output missing
	exists, err = verifier.VerifyOutputs(tmpDir, []string{"out1.txt", "missing.txt"})
	require.NoError(t, err)
	assert.False(t, exists)

	// Case 3: Error during stat (e.g., permission denied)
	// This is hard to simulate reliably across OSes without root, but we can try making a directory unreadable
	// or just skip this for now as IsNotExist is the main path.
}

func TestVerifier_VerifyOutputs_StatError(t *testing.T) {
	tmpDir := t.TempDir()
	verifier := fs.NewVerifier()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0o700))

	// Create a file in the subdirectory
	testFile := filepath.Join(subDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0o600))

	// Make the subdirectory unreadable (this will cause stat to fail with permission denied)
	require.NoError(t, os.Chmod(subDir, 0o000))
	defer func() {
		_ = os.Chmod(subDir, 0o700) //nolint:gosec // Restore permissions for cleanup
	}()

	// Try to verify the file - should return error (not just false)
	exists, err := verifier.VerifyOutputs(tmpDir, []string{"subdir/test.txt"})
	assert.False(t, exists)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat output")
}
