// Package nix implements the DependencyResolver port for resolving tool versions via NixHub.
package nix

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/zerr"
)

const (
	nixHubAPIBase     = "https://search.devbox.sh/v2/resolve"
	httpClientTimeout = 30 * time.Second
)

var supportedSystems = map[string]struct{}{
	"x86_64-linux":   {},
	"aarch64-linux":  {},
	"x86_64-darwin":  {},
	"aarch64-darwin": {},
}

// Resolver implements ports.DependencyResolver using NixHub API with local caching.
type Resolver struct {
	cacheDir   string
	httpClient *http.Client
}

// NewResolver creates a new DependencyResolver backed by NixHub API.
func NewResolver() (*Resolver, error) {
	return newResolverWithPath(domain.DefaultNixHubCachePath())
}

// newResolverWithPath creates a Resolver with a custom cache path (used for testing).
func newResolverWithPath(path string) (*Resolver, error) {
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(cleanPath, domain.DirPerm); err != nil {
		return nil, zerr.Wrap(err, domain.ErrNixCacheCreateFailed.Error())
	}

	return &Resolver{
		cacheDir: cleanPath,
		httpClient: &http.Client{
			Timeout: httpClientTimeout,
		},
	}, nil
}

// newResolverWithClient creates a Resolver with a custom http client and cache path (used for testing).
func newResolverWithClient(path string, client *http.Client) (*Resolver, error) {
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(cleanPath, domain.DirPerm); err != nil {
		return nil, zerr.Wrap(err, domain.ErrNixCacheCreateFailed.Error())
	}

	return &Resolver{
		cacheDir:   cleanPath,
		httpClient: client,
	}, nil
}

// Resolve resolves a tool alias and version to a Nixpkgs commit hash and attribute path.
// It checks the cache first, then queries the NixHub API if needed.
func (r *Resolver) Resolve(ctx context.Context, alias, version string) (commitHash, attrPath string, err error) {
	// Detect current system
	system := getCurrentSystem()

	// Try to load from cache first
	cachePath := r.getCachePath(alias, version)
	commitHash, attrPath, err = r.loadFromCache(cachePath, system)
	if err == nil {
		return commitHash, attrPath, nil
	}

	// Cache miss, query NixHub API
	apiResponse, err := r.queryNixHub(ctx, alias, version)
	if err != nil {
		return "", "", err
	}

	// Extract commit hash for current system
	systemData, ok := apiResponse.Systems[system]
	if !ok {
		unsupportedErr := zerr.With(domain.ErrNixPackageNotFound, "alias", alias)
		unsupportedErr = zerr.With(unsupportedErr, "version", version)
		return "", "", zerr.With(unsupportedErr, "system", system)
	}
	commitHash = systemData.FlakeInstallable.Ref.Rev
	attrPath = systemData.FlakeInstallable.AttrPath

	// Save to cache for future use
	if err := r.saveToCache(cachePath, alias, version, apiResponse); err != nil {
		// Log warning but don't fail the resolution
		// The cache write failure is not critical
		_ = err
	}

	return commitHash, attrPath, nil
}

// getHash generates a SHA-256 hash from a tool name and version.
// This is used to create deterministic identifiers for tool/version combinations.
func getHash(toolName, version string) string {
	input := toolName + "@" + version
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// getCachePath returns the file path for the cache entry.
func (r *Resolver) getCachePath(alias, version string) string {
	hash := getHash(alias, version)
	return filepath.Join(r.cacheDir, hash+".json")
}

// loadFromCache attempts to load a cached resolution result for the given system.
func (r *Resolver) loadFromCache(path, system string) (commitHash, attrPath string, err error) {
	//nolint:gosec // Path is constructed from trusted directory and hashed filename
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", domain.ErrNixCacheReadFailed
		}
		return "", "", zerr.Wrap(err, domain.ErrNixCacheReadFailed.Error())
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", "", zerr.Wrap(err, domain.ErrNixCacheUnmarshalFailed.Error())
	}

	// Check if system data exists in cache
	systemCache, ok := entry.Systems[system]
	if !ok {
		return "", "", domain.ErrNixCacheReadFailed
	}

	return systemCache.FlakeInstallable.Ref.Rev, systemCache.FlakeInstallable.AttrPath, nil
}

// saveToCache saves a resolution result to the cache.
func (r *Resolver) saveToCache(path, alias, version string, apiResponse *nixHubResponse) error {
	// Convert API response to cache entry
	systems := make(map[string]SystemCache)
	for sysName, sysData := range apiResponse.Systems {
		if _, supported := supportedSystems[sysName]; !supported {
			continue
		}
		systems[sysName] = SystemCache{
			FlakeInstallable: sysData.FlakeInstallable,
			Outputs:          sysData.Outputs,
		}
	}

	entry := cacheEntry{
		Alias:     alias,
		Version:   version,
		Systems:   systems,
		Timestamp: time.Now(),
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return zerr.Wrap(err, domain.ErrNixCacheMarshalFailed.Error())
	}

	if err := r.atomicWriteFile(path, data); err != nil {
		return zerr.Wrap(err, domain.ErrNixCacheWriteFailed.Error())
	}

	return nil
}

// atomicWriteFile writes data to a file atomically by writing to a temp file and renaming it.
func (r *Resolver) atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, domain.DirPerm); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, "resolver-cache-*.json")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()

	// Clean up temp file on error
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, domain.FilePerm); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}

// queryNixHub queries the NixHub API to resolve a package version.
func (r *Resolver) queryNixHub(ctx context.Context, alias, version string) (*nixHubResponse, error) {
	url := fmt.Sprintf("%s?name=%s&version=%s", nixHubAPIBase, alias, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, zerr.Wrap(err, domain.ErrNixAPIRequestFailed.Error())
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, zerr.Wrap(err, domain.ErrNixAPIRequestFailed.Error())
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log or handle close error if needed
			_ = closeErr
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		notFoundErr := zerr.With(domain.ErrNixPackageNotFound, "alias", alias)
		return nil, zerr.With(notFoundErr, "version", version)
	}

	if resp.StatusCode != http.StatusOK {
		apiErr := zerr.With(domain.ErrNixAPIRequestFailed, "status_code", resp.StatusCode)
		apiErr = zerr.With(apiErr, "alias", alias)
		return nil, zerr.With(apiErr, "version", version)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, zerr.Wrap(err, domain.ErrNixAPIRequestFailed.Error())
	}

	var apiResp nixHubResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, zerr.Wrap(err, domain.ErrNixAPIParseFailed.Error())
	}

	if len(apiResp.Systems) == 0 {
		noSystemsErr := zerr.With(domain.ErrNixPackageNotFound, "alias", alias)
		return nil, zerr.With(noSystemsErr, "version", version)
	}

	return &apiResp, nil
}

// getCurrentSystem returns the current system architecture string in NixHub format.
func getCurrentSystem() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map Go's GOOS/GOARCH to NixHub system strings
	switch {
	case goos == "darwin" && goarch == "amd64":
		return "x86_64-darwin"
	case goos == "darwin" && goarch == "arm64":
		return "aarch64-darwin"
	case goos == "linux" && goarch == "amd64":
		return "x86_64-linux"
	case goos == "linux" && goarch == "arm64":
		return "aarch64-linux"
	default:
		// Fallback to x86_64-linux for unknown systems
		return "x86_64-linux"
	}
}
