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
