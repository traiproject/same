package nix_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/nix"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestParseNixDevEnv(t *testing.T) {
	// Sample JSON output from nix print-dev-env
	sampleJSON := []byte(`{
		"variables": {
			"PATH": {
				"type": "array",
				"value": ["/nix/store/abc-go/bin", "/nix/store/xyz-git/bin"]
			},
			"GOROOT": {
				"type": "string",
				"value": "/nix/store/abc-go/share/go"
			},
			"TERM": {
				"type": "string",
				"value": "xterm-256color"
			},
			"UNKNOWN_VAR": {
				"type": "unknown_type",
				"value": 123
			}
		}
	}`)

	env, err := nix.ParseNixDevEnv(sampleJSON)
	require.NoError(t, err)

	// Verify allowed variables are present and formatted correctly
	assert.Contains(t, env, "PATH=/nix/store/abc-go/bin:/nix/store/xyz-git/bin")
	assert.Contains(t, env, "GOROOT=/nix/store/abc-go/share/go")

	// Verify blacklisted variables (TERM) are excluded
	for _, e := range env {
		assert.NotContains(t, e, "TERM=")
	}

	// Verify unknown types are skipped
	for _, e := range env {
		assert.NotContains(t, e, "UNKNOWN_VAR=")
	}
}

func TestEnvFactory_GetEnvironment_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Strict mock: EXPECT NO CALLS to Resolve
	mockResolver := mocks.NewMockDependencyResolver(ctrl)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(mockResolver, tmpDir)

	tools := map[string]string{
		"go": "go@1.21.0",
	}

	// 1. Manually populate cache
	envID := domain.GenerateEnvID(tools)
	cachePath := filepath.Join(tmpDir, "environments", envID+".json")

	// Create cache contents
	cachedEnv := []string{"CACHED_VAR=123", "GOTOOLCHAIN=local"}
	data, err := json.Marshal(cachedEnv)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o700))
	require.NoError(t, os.WriteFile(cachePath, data, 0o600))

	// 2. Call GetEnvironment
	env, err := factory.GetEnvironment(context.Background(), tools)
	require.NoError(t, err)

	// 3. Verify returned environment matches cache (plus standard injections)
	assert.Contains(t, env, "CACHED_VAR=123")
	assert.Contains(t, env, "GOTOOLCHAIN=local")
	// Check injection of TMPDIR
	foundTmp := false
	for _, e := range env {
		if len(e) >= 7 && e[:7] == "TMPDIR=" {
			foundTmp = true
			break
		}
	}
	assert.True(t, foundTmp, "TMPDIR should be injected")
}
