package nix_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.trai.ch/bob/internal/adapters/nix"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestNewEnvFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	telemetry := mocks.NewMockTelemetry(ctrl)

	factory := nix.NewEnvFactoryWithCache(resolver, telemetry, "/tmp/cache")
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
	telemetry := mocks.NewMockTelemetry(ctrl)
	vertex := mocks.NewMockVertex(ctrl)

	// Expect telemetry recording
	telemetry.EXPECT().Record(gomock.Any(), "Setup Environment").Return(ctx, vertex)
	vertex.EXPECT().Stderr().Return(nil) // Discard output for test
	vertex.EXPECT().Complete(nil)

	// Use a real nixpkgs commit that should work
	resolver.EXPECT().
		Resolve(gomock.Any(), "hello", "2.12.1").
		Return("2788904d26dda6cfa1921c5abb7a2466ffe3cb8c", "pkgs.hello", nil)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(resolver, telemetry, tmpDir)

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

	verifyEnvVars(t, env)
}

func verifyEnvVars(t *testing.T, env []string) {
	t.Helper()
	t.Run("Includes PATH", func(t *testing.T) {
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
	})

	t.Run("Overrides TMPDIR", func(t *testing.T) {
		foundOne := false
		for _, envVar := range env {
			if strings.HasPrefix(envVar, "TMPDIR=") {
				foundOne = true
				val := strings.TrimPrefix(envVar, "TMPDIR=")
				if val != "/tmp" {
					t.Errorf("GetEnvironment() TMPDIR = %q, want \"/tmp\"", val)
				}
			}
		}
		if !foundOne {
			t.Error("GetEnvironment() did not include TMPDIR override")
		}
	})
}

func TestGetEnvironment_InvalidSpec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	telemetry := mocks.NewMockTelemetry(ctrl)
	vertex := mocks.NewMockVertex(ctrl)

	// Expect telemetry recording (since it starts before validation inside Do)
	telemetry.EXPECT().Record(gomock.Any(), "Setup Environment").Return(context.Background(), vertex)
	// We expect Complete with error because resolving tools fails
	vertex.EXPECT().Complete(gomock.Any())

	factory := nix.NewEnvFactoryWithCache(resolver, telemetry, "/tmp/cache")
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

func TestGetEnvironment_AliasMismatch(t *testing.T) {
	// This test requires nix to be installed and will actually run nix build
	// Skip if nix is not available
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not found in PATH, skipping integration test")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	telemetry := mocks.NewMockTelemetry(ctrl)
	vertex := mocks.NewMockVertex(ctrl)

	// Expect telemetry recording
	telemetry.EXPECT().Record(gomock.Any(), "Setup Environment").Return(context.Background(), vertex)
	vertex.EXPECT().Stderr().Return(nil)
	// Expect failure or success depending on test, but likely failure due to bug or success
	vertex.EXPECT().Complete(gomock.Any())

	// Expect resolution with the PACKAGE NAME "golangci-lint", not the alias "lint"
	resolver.EXPECT().
		Resolve(gomock.Any(), "golangci-lint", "2.6.2").
		Return("2788904d26dda6cfa1921c5abb7a2466ffe3cb8c", "pkgs.hello", nil)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(resolver, telemetry, tmpDir)
	ctx := context.Background()

	tools := map[string]string{
		"lint": "golangci-lint@2.6.2",
	}

	// This should succeed if the implementation uses the package name
	// It will fail if it uses the alias "lint" because the mock expects "golangci-lint"
	_, err := factory.GetEnvironment(ctx, tools)

	// We expect failure currently because the implementation is buggy, but for the test itself
	// in the "Fix" phase, we want to assert success.
	// However, since I am adding this BEFORE the fix, if run now it should fail.
	// The plan says "Create reproduction test case", then "Apply fix".
	// So I will add it expecting success, and then running it will show failure.
	// We expect failure currently because the implementation is buggy, but for the test itself
	// in the "Fix" phase, we want to assert success.
	// The real check is below.
	_ = err // Ignore error for potential compilation unused var check if I decide to not check it strictly yet?
	// actually let's check it strictly.
	if err != nil {
		t.Fatalf("GetEnvironment() failed: %v", err)
	}
}

func TestGetEnvironment_ResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	telemetry := mocks.NewMockTelemetry(ctrl)
	vertex := mocks.NewMockVertex(ctrl)

	// Expect telemetry recording
	telemetry.EXPECT().Record(gomock.Any(), "Setup Environment").Return(context.Background(), vertex)
	vertex.EXPECT().Complete(gomock.Any())

	// Mock resolver to return error
	resolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.25.4").
		Return("", "", fmt.Errorf("resolver error"))

	factory := nix.NewEnvFactoryWithCache(resolver, telemetry, "/tmp/cache")
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
		{"NIX_ prefix included", "NIX_rando", true},
		{"NIX_BUILD_CORES excluded", "NIX_BUILD_CORES", false},
		{"TERM excluded", "TERM", false},
		{"SHELL excluded", "SHELL", false},
		{"HOME excluded", "HOME", false},
		{"random var included", "MY_CUSTOM_VAR", true},
		{"TMPDIR excluded", "TMPDIR", false},
		{"TEMP excluded", "TEMP", false},
		{"TMP excluded", "TMP", false},
		{"NIX_BUILD_TOP excluded", "NIX_BUILD_TOP", false},
		{"NIX_LOG_FD excluded", "NIX_LOG_FD", false},
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

func TestGetEnvironment_Concurrency(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	telemetry := mocks.NewMockTelemetry(ctrl)
	vertex := mocks.NewMockVertex(ctrl)

	// Since we are running twice concurrently, we might get one or two telemetry records
	// because singleflight dedupes calls.
	// However, one of them will perform the work and record telemetry.
	// The other waits. The singleflight wrapper returns the result to both.
	// The current implementation calls Record INSIDE the singleflight Do.
	// So it should only be called ONCE.
	telemetry.EXPECT().Record(gomock.Any(), "Setup Environment").Return(context.Background(), vertex).Times(1)
	vertex.EXPECT().Complete(nil).Times(1)
	vertex.EXPECT().Stderr().Return(nil).Times(1)

	// Use atomic counter to verify single execution
	var callCount int32

	// Mock Resolve to sleep to ensure overlaps
	resolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.25.4").
		DoAndReturn(func(_ context.Context, _, _ string) (string, string, error) {
			atomic.AddInt32(&callCount, 1)
			time.Sleep(100 * time.Millisecond) // Simulate work to ensure other goroutines blocked
			return "2788904d26dda6cfa1921c5abb7a2466ffe3cb8c", "pkgs.go", nil
		}).
		Times(1)

	tmpDir := t.TempDir()
	factory := nix.NewEnvFactoryWithCache(resolver, telemetry, tmpDir)
	ctx := context.Background()

	tools := map[string]string{
		"go": "go@1.25.4",
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Start two concurrent requests
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, _ = factory.GetEnvironment(ctx, tools)
		}()
	}

	wg.Wait()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Resolve called %d times, want 1", callCount)
	}
}

func TestSaveEnvToCache_WriteError(t *testing.T) {
	// Use a read-only directory
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0o500); err != nil {
		t.Fatalf("Failed to create readonly dir: %v", err)
	}

	// Try to save to a file inside the read-only directory
	// In Unix, you need write permission on the directory to create files
	cachePath := filepath.Join(readOnlyDir, "test.json")
	env := []string{"PATH=/test"}

	err := nix.SaveEnvToCache(cachePath, env)
	if err == nil {
		t.Error("SaveEnvToCache() expected error for read-only directory")
	}
}

func TestGetEnvironment_PartialResolveFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	telemetry := mocks.NewMockTelemetry(ctrl)
	vertex := mocks.NewMockVertex(ctrl)

	// Expect telemetry recording
	telemetry.EXPECT().Record(gomock.Any(), "Setup Environment").Return(context.Background(), vertex)
	vertex.EXPECT().Complete(gomock.Any())

	// Mock one success and one failure
	resolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.25.4").
		Return("hash-go", "pkgs.go", nil)

	resolver.EXPECT().
		Resolve(gomock.Any(), "golangci-lint", "2.6.2").
		Return("", "", errors.New("resolution failed"))

	factory := nix.NewEnvFactoryWithCache(resolver, telemetry, "/tmp/cache")
	ctx := context.Background()

	tools := map[string]string{
		"go":            "go@1.25.4",
		"golangci-lint": "golangci-lint@2.6.2",
	}

	_, err := factory.GetEnvironment(ctx, tools)
	if err == nil {
		t.Error("GetEnvironment() expected error when one tool fails resolution")
	}
	if !strings.Contains(err.Error(), "resolution failed") {
		t.Errorf("expected error containing 'resolution failed', got %v", err)
	}
}
