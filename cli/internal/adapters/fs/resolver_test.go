package fs_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/fs"
	"go.trai.ch/same/internal/core/domain"
)

func TestResolver_ResolveInputs(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	mustCreateFile(t, rootDir, "main.go")
	mustCreateFile(t, rootDir, "utils.go")
	mustCreateDir(t, rootDir, "pkg")
	mustCreateFile(t, rootDir, "pkg/lib.go")
	mustCreateDir(t, rootDir, "other")
	mustCreateFile(t, rootDir, "other/README.md")

	resolver := fs.NewResolver()

	t.Run("globs", func(t *testing.T) {
		t.Parallel()
		inputs := []string{"*.go", "pkg/*.go"}
		resolved, err := resolver.ResolveInputs(inputs, rootDir)
		require.NoError(t, err)

		expected := []string{
			filepath.Join(rootDir, "main.go"),
			filepath.Join(rootDir, "pkg", "lib.go"),
			filepath.Join(rootDir, "utils.go"),
		}
		assert.Equal(t, expected, resolved)
	})

	t.Run("deduplication", func(t *testing.T) {
		t.Parallel()
		inputs := []string{"*.go", "main.go"}
		resolved, err := resolver.ResolveInputs(inputs, rootDir)
		require.NoError(t, err)

		expected := []string{
			filepath.Join(rootDir, "main.go"),
			filepath.Join(rootDir, "utils.go"),
		}
		assert.Equal(t, expected, resolved)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		inputs := []string{"*.java"} // doesn't exist
		_, err := resolver.ResolveInputs(inputs, rootDir)
		require.Error(t, err)
		assert.ErrorContains(t, err, domain.ErrInputNotFound.Error())
	})

	t.Run("one found one missing", func(t *testing.T) {
		t.Parallel()
		// If one matches and one doesn't, does it fail?
		// The current implementation loops and fails as soon as one is not found.
		inputs := []string{"*.go", "*.java"}
		_, err := resolver.ResolveInputs(inputs, rootDir)
		require.Error(t, err)
		assert.ErrorContains(t, err, domain.ErrInputNotFound.Error())
	})
}
