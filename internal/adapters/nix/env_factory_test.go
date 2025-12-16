package nix_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.trai.ch/bob/internal/adapters/nix"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestNewEnvFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	factory := nix.NewEnvFactoryWithCache(resolver, manager, "/tmp/cache")
	if factory == nil {
		t.Fatal("NewEnvFactory() returned nil")
	}
}

func TestGetEnvironment_Success(t *testing.T) {
	// This test requires nix to be installed and will actually run nix build
	// Skip if nix is not available
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not found in PATH, skipping integration test")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	// Use a real nixpkgs commit that should work
	resolver.EXPECT().
		Resolve(gomock.Any(), "hello", "2.12.1").
		Return("2788904d26dda6cfa1921c5abb7a2466ffe3cb8c", nil)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(resolver, manager, tmpDir)

	tools := map[string]string{
		"hello": "hello@2.12.1",
	}

	env, err := factory.GetEnvironment(ctx, tools)
	if err != nil {
		t.Fatalf("GetEnvironment() error = %v", err)
	}

	if env == nil {
		t.Error("GetEnvironment() returned nil environment")
	}

	if len(env) == 0 {
		t.Error("GetEnvironment() returned empty environment")
	}

	// Check that PATH is included
	hasPath := false
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "PATH=") {
			hasPath = true
			break
		}
	}
	if !hasPath {
		t.Error("GetEnvironment() did not include PATH variable")
	}
}

func TestGetEnvironment_InvalidSpec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	factory := nix.NewEnvFactoryWithCache(resolver, manager, "/tmp/cache")
	ctx := context.Background()

	tools := map[string]string{
		"go": "invalid-spec-without-at",
	}

	_, err := factory.GetEnvironment(ctx, tools)
	if err == nil {
		t.Error("GetEnvironment() expected error for invalid spec")
	}

	if !strings.Contains(err.Error(), "invalid tool spec format") {
		t.Errorf("GetEnvironment() error = %v, want error containing 'invalid tool spec format'", err)
	}
}

func TestGetEnvironment_ResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	// Mock resolver to return error
	resolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.25.4").
		Return("", fmt.Errorf("resolver error"))

	factory := nix.NewEnvFactoryWithCache(resolver, manager, "/tmp/cache")
	ctx := context.Background()

	tools := map[string]string{
		"go": "go@1.25.4",
	}

	_, err := factory.GetEnvironment(ctx, tools)
	if err == nil {
		t.Error("GetEnvironment() expected error when resolver fails")
	}
}

func TestGenerateEnvID(t *testing.T) {
	t.Run("deterministic hash", func(t *testing.T) {
		tools := map[string]string{
			"go":            "go@1.25.4",
			"golangci-lint": "golangci-lint@2.6.2",
		}

		id1 := domain.GenerateEnvID(tools)
		id2 := domain.GenerateEnvID(tools)

		if id1 != id2 {
			t.Errorf("generateEnvID() not deterministic: %s != %s", id1, id2)
		}

		if len(id1) != 64 { // SHA-256 hex string
			t.Errorf("generateEnvID() length = %d, want 64", len(id1))
		}
	})

	t.Run("different tools produce different hashes", func(t *testing.T) {
		tools1 := map[string]string{
			"go": "go@1.25.4",
		}
		tools2 := map[string]string{
			"go": "go@1.24.0",
		}

		id1 := domain.GenerateEnvID(tools1)
		id2 := domain.GenerateEnvID(tools2)

		if id1 == id2 {
			t.Error("generateEnvID() produced same hash for different tools")
		}
	})

	t.Run("order independent", func(t *testing.T) {
		tools1 := map[string]string{
			"go":            "go@1.25.4",
			"golangci-lint": "golangci-lint@2.6.2",
		}
		tools2 := map[string]string{
			"golangci-lint": "golangci-lint@2.6.2",
			"go":            "go@1.25.4",
		}

		id1 := domain.GenerateEnvID(tools1)
		id2 := domain.GenerateEnvID(tools2)

		if id1 != id2 {
			t.Errorf("generateEnvID() not order independent: %s != %s", id1, id2)
		}
	})
}

func TestCacheLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test.json")

	env := []string{
		"PATH=/nix/store/xyz/bin",
		"GOROOT=/nix/store/abc/go",
	}

	// Test save
	err := nix.SaveEnvToCache(cachePath, env)
	if err != nil {
		t.Fatalf("saveEnvToCache() error = %v", err)
	}

	// Verify file exists
	if _, statErr := os.Stat(cachePath); os.IsNotExist(statErr) {
		t.Error("saveEnvToCache() did not create cache file")
	}

	// Test load
	loaded, err := nix.LoadEnvFromCache(cachePath)
	if err != nil {
		t.Fatalf("loadEnvFromCache() error = %v", err)
	}

	if len(loaded) != len(env) {
		t.Errorf("loadEnvFromCache() length = %d, want %d", len(loaded), len(env))
	}

	for i, v := range env {
		if loaded[i] != v {
			t.Errorf("loadEnvFromCache()[%d] = %s, want %s", i, loaded[i], v)
		}
	}
}

func TestCacheLoadMiss(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "nonexistent.json")

	_, err := nix.LoadEnvFromCache(cachePath)
	if err == nil {
		t.Error("loadEnvFromCache() expected error for missing file")
	}
}

func TestParseNixDevEnv(t *testing.T) {
	t.Run("success with string values", func(t *testing.T) {
		jsonData := []byte(`{
			"variables": {
				"PATH": {"type": "exported", "value": "/nix/store/xyz/bin"},
				"GOROOT": {"type": "exported", "value": "/nix/store/abc/go"},
				"TERM": {"type": "exported", "value": "xterm"}
			}
		}`)

		env, err := nix.ParseNixDevEnv(jsonData)
		if err != nil {
			t.Fatalf("parseNixDevEnv() error = %v", err)
		}

		// Should include PATH and GOROOT, but exclude TERM
		checkEnvVars(t, env, []string{"PATH", "GOROOT"}, []string{"TERM"})
	})

	t.Run("success with array values", func(t *testing.T) {
		jsonData := []byte(`{
			"variables": {
				"PATH": {"type": "exported", "value": ["/nix/store/a/bin", "/nix/store/b/bin"]}
			}
		}`)

		env, err := nix.ParseNixDevEnv(jsonData)
		if err != nil {
			t.Fatalf("parseNixDevEnv() error = %v", err)
		}

		found := false
		for _, envVar := range env {
			if strings.HasPrefix(envVar, "PATH=") {
				found = true
				expected := "PATH=/nix/store/a/bin:/nix/store/b/bin"
				if envVar != expected {
					t.Errorf("parseNixDevEnv() PATH = %s, want %s", envVar, expected)
				}
			}
		}

		if !found {
			t.Error("parseNixDevEnv() missing PATH")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		jsonData := []byte(`invalid json`)

		_, err := nix.ParseNixDevEnv(jsonData)
		if err == nil {
			t.Error("parseNixDevEnv() expected error for invalid JSON")
		}
	})
}

// checkEnvVars is a helper that checks for presence/absence of environment variables.
func checkEnvVars(t *testing.T, env, shouldInclude, shouldExclude []string) {
	t.Helper()

	for _, key := range shouldInclude {
		found := false
		for _, envVar := range env {
			if strings.HasPrefix(envVar, key+"=") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("parseNixDevEnv() missing %s", key)
		}
	}

	for _, key := range shouldExclude {
		for _, envVar := range env {
			if strings.HasPrefix(envVar, key+"=") {
				t.Errorf("parseNixDevEnv() should not include %s", key)
			}
		}
	}
}

func TestShouldIncludeVar(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"PATH included", "PATH", true},
		{"GOROOT included", "GOROOT", true},
		{"CC included", "CC", true},
		{"NIX_ prefix included", "NIX_BUILD_CORES", true},
		{"TERM excluded", "TERM", false},
		{"SHELL excluded", "SHELL", false},
		{"HOME excluded", "HOME", false},
		{"random var excluded", "MY_CUSTOM_VAR", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nix.ShouldIncludeVar(tt.key)
			if result != tt.expected {
				t.Errorf("shouldIncludeVar(%s) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestLoadEnvFromCache_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON to cache file
	invalidJSON := []byte(`{"invalid": json}`)
	if err := os.WriteFile(cachePath, invalidJSON, 0o600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := nix.LoadEnvFromCache(cachePath)
	if err == nil {
		t.Error("LoadEnvFromCache() expected error for invalid JSON")
	}
}

func TestSaveEnvToCache_MkdirError(t *testing.T) {
	// Try to create a cache file in a path that cannot be created
	// Use a file as parent instead of directory
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking")

	// Create a file that blocks directory creation
	if err := os.WriteFile(blockingFile, []byte("block"), 0o600); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	// Try to save cache in a subdirectory of the file (should fail)
	invalidPath := filepath.Join(blockingFile, "subdir", "cache.json")
	env := []string{"PATH=/test"}

	err := nix.SaveEnvToCache(invalidPath, env)
	if err == nil {
		t.Error("SaveEnvToCache() expected error when directory cannot be created")
	}
}
