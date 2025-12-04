//nolint:testpackage // Testing internal functions like getHash
package nix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.trai.ch/bob/internal/core/domain"
)

const testVersion = "1.21"

func TestNewResolver(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache")

	resolver, err := newResolverWithPath(cachePath)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	if resolver == nil {
		t.Fatal("NewResolver() returned nil resolver")
	}

	// Verify cache directory was created
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Errorf("cache directory was not created")
	}
}

func TestGetHash(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		version   string
		wantSame  bool
		compareTo struct {
			toolName string
			version  string
		}
	}{
		{
			name:     "deterministic hash",
			toolName: "go",
			version:  "1.21",
			wantSame: true,
			compareTo: struct {
				toolName string
				version  string
			}{
				toolName: "go",
				version:  "1.21",
			},
		},
		{
			name:     "different version produces different hash",
			toolName: "go",
			version:  "1.21",
			wantSame: false,
			compareTo: struct {
				toolName string
				version  string
			}{
				toolName: "go",
				version:  "1.22",
			},
		},
		{
			name:     "different tool produces different hash",
			toolName: "go",
			version:  "1.21",
			wantSame: false,
			compareTo: struct {
				toolName string
				version  string
			}{
				toolName: "node",
				version:  "1.21",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := getHash(tt.toolName, tt.version)
			hash2 := getHash(tt.compareTo.toolName, tt.compareTo.version)

			if tt.wantSame && hash1 != hash2 {
				t.Errorf("expected same hash, got %s and %s", hash1, hash2)
			}
			if !tt.wantSame && hash1 == hash2 {
				t.Errorf("expected different hash, got %s", hash1)
			}

			// Verify hash is hex encoded SHA-256 (64 characters)
			if len(hash1) != 64 {
				t.Errorf("hash length = %d, want 64", len(hash1))
			}
		})
	}
}

func TestResolve_FromCache(t *testing.T) {
	tmpDir := t.TempDir()
	resolver, err := newResolverWithPath(tmpDir)
	if err != nil {
		t.Fatalf("newResolverWithPath() error = %v", err)
	}

	// Pre-populate cache
	alias := "go"
	version := testVersion
	expectedHash := "abc123def456"
	cachePath := resolver.getCachePath(alias, version)

	// Create cache entry with all systems
	systems := map[string]SystemCache{
		"x86_64-linux": {
			CommitHash: expectedHash,
			AttrPath:   "legacyPackages.x86_64-linux.go",
		},
		"aarch64-linux": {
			CommitHash: expectedHash,
			AttrPath:   "legacyPackages.aarch64-linux.go",
		},
		"x86_64-darwin": {
			CommitHash: expectedHash,
			AttrPath:   "legacyPackages.x86_64-darwin.go",
		},
		"aarch64-darwin": {
			CommitHash: expectedHash,
			AttrPath:   "legacyPackages.aarch64-darwin.go",
		},
	}

	entry := cacheEntry{
		Alias:    alias,
		Version:  version,
		Systems:  systems,
		CachedAt: time.Now(),
	}
	data, _ := json.MarshalIndent(entry, "", "  ")
	if writeErr := os.WriteFile(cachePath, data, filePerm); writeErr != nil {
		t.Fatalf("failed to write cache: %v", writeErr)
	}

	// Resolve should return cached value without hitting API
	ctx := context.Background()
	got, err := resolver.Resolve(ctx, alias, version)
	if err != nil {
		t.Errorf("Resolve() error = %v", err)
	}
	if got != expectedHash {
		t.Errorf("Resolve() = %v, want %v", got, expectedHash)
	}
}

func TestResolve_CacheMiss_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock NixHub API server
	expectedHash := "test-commit-hash-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "go" || r.URL.Query().Get("version") != testVersion {
			t.Errorf("unexpected query params: %v", r.URL.Query())
		}

		resp := nixHubResponse{
			Name:    "go",
			Version: testVersion,
			Summary: "The Go programming language",
			Systems: map[string]SystemResponse{
				"x86_64-linux": {
					FlakeInstallable: FlakeInstallable{
						Ref: FlakeRef{
							Type:  "github",
							Owner: "NixOS",
							Repo:  "nixpkgs",
							Rev:   expectedHash,
						},
						AttrPath: "legacyPackages.x86_64-linux.go",
					},
				},
				"aarch64-darwin": {
					FlakeInstallable: FlakeInstallable{
						Ref: FlakeRef{
							Type:  "github",
							Owner: "NixOS",
							Repo:  "nixpkgs",
							Rev:   expectedHash,
						},
						AttrPath: "legacyPackages.aarch64-darwin.go",
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create resolver with custom HTTP client that uses our test server
	resolver := &Resolver{
		cacheDir: tmpDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &testTransport{
				serverURL: server.URL,
			},
		},
	}

	ctx := context.Background()
	got, err := resolver.Resolve(ctx, "go", testVersion)
	if err != nil {
		t.Errorf("Resolve() error = %v", err)
	}
	if got != expectedHash {
		t.Errorf("Resolve() = %v, want %v", got, expectedHash)
	}

	// Verify cache was written
	cachePath := filepath.Join(tmpDir, getHash("go", testVersion)+".json")
	//nolint:gosec // Test file path is controlled
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Errorf("cache file not written: %v", err)
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Errorf("invalid cache data: %v", err)
	}

	// Verify at least one system is cached
	if len(entry.Systems) == 0 {
		t.Error("no systems cached")
	}

	// Verify current system has the expected hash
	system := getCurrentSystem()
	sysCache, ok := entry.Systems[system]
	if !ok {
		t.Errorf("current system %s not found in cache", system)
	}
	if sysCache.CommitHash != expectedHash {
		t.Errorf("cached hash = %v, want %v", sysCache.CommitHash, expectedHash)
	}
}

// testTransport is a custom RoundTripper that redirects requests to a test server.
type testTransport struct {
	serverURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Preserve query parameters
	testURL := t.serverURL + "?" + req.URL.RawQuery
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, testURL, req.Body)
	if err != nil {
		return nil, err
	}
	return http.DefaultTransport.RoundTrip(newReq)
}

func TestResolve_PackageNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock server returning 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	resolver := &Resolver{
		cacheDir: tmpDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &testTransport{
				serverURL: server.URL,
			},
		},
	}

	ctx := context.Background()
	_, err := resolver.Resolve(ctx, "nonexistent", "1.0.0")
	if err == nil {
		t.Error("Resolve() expected error for nonexistent package")
	}
	// Error should be wrapped, so check if it contains ErrNixPackageNotFound
	if !strings.Contains(err.Error(), domain.ErrNixPackageNotFound.Error()) {
		t.Errorf("Resolve() error = %v, want error containing %v", err, domain.ErrNixPackageNotFound)
	}
}

func TestResolve_InvalidCacheData(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON to cache
	hash := getHash("go", testVersion)
	cachePath := filepath.Join(tmpDir, hash+".json")
	if err := os.WriteFile(cachePath, []byte("invalid json"), filePerm); err != nil {
		t.Fatalf("failed to write invalid cache: %v", err)
	}

	// Mock successful API response with all systems
	expectedHash := "fallback-hash-456"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := nixHubResponse{
			Name:    "go",
			Version: testVersion,
			Systems: map[string]SystemResponse{
				"x86_64-linux": {
					FlakeInstallable: FlakeInstallable{
						Ref: FlakeRef{
							Rev: expectedHash,
						},
						AttrPath: "legacyPackages.x86_64-linux.go",
					},
				},
				"aarch64-linux": {
					FlakeInstallable: FlakeInstallable{
						Ref: FlakeRef{
							Rev: expectedHash,
						},
						AttrPath: "legacyPackages.aarch64-linux.go",
					},
				},
				"x86_64-darwin": {
					FlakeInstallable: FlakeInstallable{
						Ref: FlakeRef{
							Rev: expectedHash,
						},
						AttrPath: "legacyPackages.x86_64-darwin.go",
					},
				},
				"aarch64-darwin": {
					FlakeInstallable: FlakeInstallable{
						Ref: FlakeRef{
							Rev: expectedHash,
						},
						AttrPath: "legacyPackages.aarch64-darwin.go",
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	resolver := &Resolver{
		cacheDir: tmpDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &testTransport{
				serverURL: server.URL,
			},
		},
	}

	// Should fall back to API when cache is corrupted
	ctx := context.Background()
	got, err := resolver.Resolve(ctx, "go", testVersion)
	if err != nil {
		t.Errorf("Resolve() error = %v", err)
	}
	if got != expectedHash {
		t.Errorf("Resolve() = %v, want %v (should fallback to API)", got, expectedHash)
	}
}

func TestLoadFromCache_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	resolver, err := newResolverWithPath(tmpDir)
	if err != nil {
		t.Fatalf("newResolverWithPath() error = %v", err)
	}

	_, err = resolver.loadFromCache(filepath.Join(tmpDir, "nonexistent.json"), "x86_64-linux")
	if err == nil {
		t.Error("loadFromCache() expected error for nonexistent file")
	}
}

func TestSaveToCache(t *testing.T) {
	tmpDir := t.TempDir()
	resolver, err := newResolverWithPath(tmpDir)
	if err != nil {
		t.Fatalf("newResolverWithPath() error = %v", err)
	}

	cachePath := filepath.Join(tmpDir, "test.json")

	// Create a mock API response to save
	apiResp := &nixHubResponse{
		Name:    "go",
		Version: testVersion,
		Systems: map[string]SystemResponse{
			"x86_64-linux": {
				FlakeInstallable: FlakeInstallable{
					Ref: FlakeRef{
						Rev: "test-hash",
					},
					AttrPath: "legacyPackages.x86_64-linux.go",
				},
			},
		},
	}

	err = resolver.saveToCache(cachePath, "go", testVersion, apiResp)
	if err != nil {
		t.Errorf("saveToCache() error = %v", err)
	}

	// Verify file was written
	//nolint:gosec // Test file path is controlled
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("invalid cache file: %v", err)
	}

	if entry.Alias != "go" {
		t.Errorf("entry.Alias = %v, want go", entry.Alias)
	}
	if entry.Version != testVersion {
		t.Errorf("entry.Version = %v, want %s", entry.Version, testVersion)
	}

	// Verify systems data
	if len(entry.Systems) == 0 {
		t.Error("entry.Systems is empty")
	}

	sys, ok := entry.Systems["x86_64-linux"]
	if !ok {
		t.Error("x86_64-linux system not found in cache")
	}
	if sys.CommitHash != "test-hash" {
		t.Errorf("sys.CommitHash = %v, want test-hash", sys.CommitHash)
	}
}
