package nix_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/nix"
	"go.trai.ch/same/internal/core/domain"
)

func TestEnvFactory_GenerateNixExpr_Golden(t *testing.T) {
	// 1. Setup Inputs
	system := "x86_64-darwin"
	commits := map[string][]string{
		"1234567890abcdef": {"go_1_25", "golangci-lint"},
		"fedcba0987654321": {"nodejs-18_x"},
	}

	// 2. Call the exposed method
	// EnvFactory structure is simple, we just need the method.
	factory := nix.NewEnvFactoryWithCache(nil, "")
	output := factory.GenerateNixExprRaw(system, commits)

	// 3. Compare with Golden File
	goldenFile := filepath.Join("testdata", "golden_env.nix")

	if os.Getenv("UPDATE_GOLDEN") == "true" {
		err := os.MkdirAll(filepath.Dir(goldenFile), domain.DirPerm)
		require.NoError(t, err)
		//nolint:gosec // Golden file needs to be readable
		err = os.WriteFile(goldenFile, []byte(output), domain.FilePerm)
		require.NoError(t, err)
	}

	//nolint:gosec // Path is constant + relative
	expected, err := os.ReadFile(goldenFile)
	if os.IsNotExist(err) {
		t.Fatalf("Golden file not found: %s. Run with UPDATE_GOLDEN=true to generate it.", goldenFile)
	}
	require.NoError(t, err)

	assert.Equal(t, string(expected), output, "Nix expression generation changed! Check if this is intentional.")
}
