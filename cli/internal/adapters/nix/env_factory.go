package nix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

// EnvFactory implements ports.EnvironmentFactory using Nix.
type EnvFactory struct {
	resolver ports.DependencyResolver

	cacheDir     string
	requestGroup singleflight.Group
}

// NewEnvFactoryWithCache creates a new EnvironmentFactory backed by Nix with a specific cache directory.
func NewEnvFactoryWithCache(
	resolver ports.DependencyResolver,
	cacheDir string,
) *EnvFactory {
	return &EnvFactory{
		resolver: resolver,
		cacheDir: cacheDir,
	}
}

// GetEnvironment constructs a hermetic environment from a set of tools.
// The tools map contains alias->spec pairs (e.g., "go" -> "go@1.25.4").
// Returns environment variables as "KEY=VALUE" strings suitable for process execution.
func (e *EnvFactory) GetEnvironment(ctx context.Context, tools map[string]string) ([]string, error) {
	// Step 0: Generate deterministic ID first to use as singleflight key
	envID := domain.GenerateEnvID(tools)

	// Wrap the entire expensive operation in singleflight to prevent cache stampedes
	result, err, _ := e.requestGroup.Do(envID, func() (any, error) {
		// Step A: Check cache first (fast path)
		cachePath := filepath.Join(e.cacheDir, "environments", envID+".json")
		if cachedEnv, err := LoadEnvFromCache(cachePath); err == nil {
			return cachedEnv, nil
		}

		// Step B: Resolve all tools to commit hashes
		commitToPackages, err := e.resolveTools(ctx, tools)
		if err != nil {
			return nil, err
		}

		// Step C: Generate and execute Nix expression
		system := getCurrentSystem()
		nixExpr := e.generateNixExpr(system, commitToPackages)

		// Write to temporary file
		tmpPath, cleanupFn, err := createNixTempFile(nixExpr)
		if err != nil {
			return nil, err
		}
		defer cleanupFn()

		// Execute nix print-dev-env
		//nolint:gosec // tmpPath is a trusted temp file created by us
		cmd := exec.CommandContext(ctx, "nix", "print-dev-env", "--json", "--file", tmpPath)
		output, err := cmd.Output()
		if err != nil {
			return nil, zerr.Wrap(err, "failed to execute nix print-dev-env")
		}

		// Parse JSON output
		env, err := ParseNixDevEnv(output)
		if err != nil {
			return nil, zerr.Wrap(err, "failed to parse nix output")
		}
		// Step D: Persist to cache

		// Enforce local toolchain for Go to prevent auto-downloading newer versions
		// based on go.mod directive.
		env = append(env, "GOTOOLCHAIN=local")

		slices.Sort(env) // Re-sort after appending

		if err := SaveEnvToCache(cachePath, env); err != nil {
			// Log warning but don't fail - cache write is not critical
			_ = err
		}

		return env, nil
	})

	if err != nil {
		return nil, err
	}

	env := slices.Clone(result.([]string))

	// Force temporary directories to local system temp to avoid leaking
	// transient build directories (e.g. from nix-shell).
	// Note: We cannot use os.TempDir() here because it respects the current TMPDIR env var,
	// which might be the polluted Nix path we are trying to avoid.
	tmpDir := "/tmp"
	env = append(env,
		fmt.Sprintf("TMPDIR=%s", tmpDir),
		fmt.Sprintf("TEMP=%s", tmpDir),
		fmt.Sprintf("TMP=%s", tmpDir),
	)

	// Also force GOCACHE to user local cache to avoid using the transient build cache
	if userCacheDir, err := os.UserCacheDir(); err == nil {
		env = append(env, fmt.Sprintf("GOCACHE=%s", filepath.Join(userCacheDir, "go-build")))
	}

	slices.Sort(env)

	return env, nil
}

// generateNixExpr generates a Nix expression from a commit-to-packages mapping.
func (e *EnvFactory) generateNixExpr(system string, commits map[string][]string) string {
	var builder strings.Builder

	// Start let block
	builder.WriteString("let\n")
	builder.WriteString(fmt.Sprintf("system = %q;\n", system))

	// Get sorted commit hashes for deterministic iteration
	commitHashes := make([]string, 0, len(commits))
	for hash := range commits {
		commitHashes = append(commitHashes, hash)
	}
	slices.Sort(commitHashes)

	// Generate flake and pkgs variables for each commit
	// We use the sorted index to ensure stability (flake_0 corresponds to first sorted commit)
	commitToIdx := make(map[string]int)

	for i, commitHash := range commitHashes {
		builder.WriteString(fmt.Sprintf("flake_%d = builtins.getFlake \"github:NixOS/nixpkgs/%s\";\n",
			i, commitHash))
		builder.WriteString(fmt.Sprintf("pkgs_%d = flake_%d.legacyPackages.${system};\n",
			i, i))
		commitToIdx[commitHash] = i
	}

	// Start mkShell block
	builder.WriteString("in\n")

	// Use the first pkgs for mkShell (arbitrary choice, all should have mkShell)
	// Since we sorted, pkgs_0 is always the first one if any exist
	firstIdx := 0

	builder.WriteString(fmt.Sprintf("pkgs_%d.mkShell {\n", firstIdx))
	builder.WriteString("buildInputs = [\n")

	// Add all packages, iterating over sorted commits first
	for _, commitHash := range commitHashes {
		idx := commitToIdx[commitHash]
		packages := commits[commitHash]

		// Sort packages for determinism
		slices.Sort(packages)

		for _, pkg := range packages {
			builder.WriteString(fmt.Sprintf("pkgs_%d.%s\n", idx, pkg))
		}
	}

	builder.WriteString("];\n")
	builder.WriteString("}\n")

	return builder.String()
}

// createNixTempFile creates a temporary file with the given Nix expression.
func createNixTempFile(nixExpr string) (tmpPath string, cleanup func(), err error) {
	tmpFile, err := os.CreateTemp("", "same-env-*.nix")
	if err != nil {
		return "", nil, zerr.Wrap(err, "failed to create temp nix file")
	}

	tmpPath = tmpFile.Name()
	cleanup = func() {
		_ = os.Remove(tmpPath)
	}

	if _, writeErr := tmpFile.WriteString(nixExpr); writeErr != nil {
		_ = tmpFile.Close()
		cleanup()
		return "", nil, zerr.Wrap(writeErr, "failed to write nix expression")
	}

	if closeErr := tmpFile.Close(); closeErr != nil {
		cleanup()
		return "", nil, zerr.Wrap(closeErr, "failed to close temp nix file")
	}

	return tmpPath, cleanup, nil
}

// LoadEnvFromCache attempts to load a cached environment.
func LoadEnvFromCache(path string) ([]string, error) {
	//nolint:gosec // Path is constructed from trusted cache directory
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, domain.ErrCacheMiss
		}
		return nil, zerr.Wrap(err, "failed to read cache file")
	}

	var env []string
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, zerr.Wrap(err, "failed to unmarshal cache")
	}

	return env, nil
}

// SaveEnvToCache saves an environment to the cache.
func SaveEnvToCache(path string, env []string) error {
	// Ensure cache directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, domain.DirPerm); err != nil {
		return zerr.Wrap(err, "failed to create cache directory")
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return zerr.Wrap(err, "failed to marshal environment")
	}

	// Create temp file in the same directory
	tmpFile, err := os.CreateTemp(dir, "env-cache-*.json")
	if err != nil {
		return zerr.Wrap(err, "failed to create temp cache file")
	}
	tmpName := tmpFile.Name()

	// Clean up temp file on error
	defer func() {
		if _, err := os.Stat(tmpName); err == nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return zerr.Wrap(err, "failed to write cache file")
	}

	if err := tmpFile.Close(); err != nil {
		return zerr.Wrap(err, "failed to close temp cache file")
	}

	// Set correct permissions
	if err := os.Chmod(tmpName, domain.FilePerm); err != nil {
		return zerr.Wrap(err, "failed to chmod cache file")
	}

	// Atomic rename
	if err := os.Rename(tmpName, path); err != nil {
		return zerr.Wrap(err, "failed to rename temp cache file")
	}

	return nil
}

// nixDevEnvOutput represents the JSON structure from `nix print-dev-env --json`.
type nixDevEnvOutput struct {
	Variables map[string]nixVariable `json:"variables"`
}

type nixVariable struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// ParseNixDevEnv parses the JSON output from nix print-dev-env and extracts environment variables.
func ParseNixDevEnv(jsonData []byte) ([]string, error) {
	var output nixDevEnvOutput
	if err := json.Unmarshal(jsonData, &output); err != nil {
		return nil, zerr.Wrap(err, "failed to unmarshal nix output")
	}

	env := make([]string, 0, len(output.Variables))
	for key, variable := range output.Variables {
		// Only include variables we want
		if !ShouldIncludeVar(key) {
			continue
		}

		// Extract value based on type
		var valueStr string
		switch v := variable.Value.(type) {
		case string:
			valueStr = v
		case []interface{}:
			// For arrays, join with colons (common for PATH-like vars)
			parts := make([]string, len(v))
			for i, part := range v {
				if s, ok := part.(string); ok {
					parts[i] = s
				}
			}
			valueStr = strings.Join(parts, ":")
		default:
			// Skip other types
			continue
		}

		env = append(env, fmt.Sprintf("%s=%s", key, valueStr))
	}

	// Sort for deterministic output
	slices.Sort(env)
	return env, nil
}

// ShouldIncludeVar determines if an environment variable should be included.
// We want to include build-related variables but exclude interactive shell variables.
func ShouldIncludeVar(key string) bool {
	// Exclude interactive shell variables and user-specific variables
	// We want to preserve the system's values for these (e.g. HOME) or rely on defaults.
	exclude := []string{
		"TERM",
		"SHELL",
		"EDITOR",
		"VISUAL",
		"PAGER",
		"LESS",
		"HOME",
		"USER",
		"LOGNAME",
		"PS1",
		"PS2",
		"SHLVL",
		"PWD",
		"OLDPWD",
		"_",
		"TMPDIR",
		"TEMP",
		"TMP",
		"NIX_BUILD_TOP",
		"NIX_BUILD_CORES",
		"NIX_LOG_FD",
	}

	return !slices.Contains(exclude, key)
}

// resolveTools resolves all tools to their commit hashes and attribute paths.
// It uses an error group to resolve tools concurrently.
func (e *EnvFactory) resolveTools(ctx context.Context, tools map[string]string) (map[string][]string, error) {
	commitToPackages := make(map[string][]string)
	var mu sync.Mutex

	g, groupCtx := errgroup.WithContext(ctx)
	// Use number of CPUs as concurrency limit, matching scheduler default
	g.SetLimit(runtime.NumCPU())

	for _, spec := range tools {
		spec := spec // Capture loop variables
		g.Go(func() error {
			// Parse spec to get package name and version
			// Spec format: "package@version" (e.g., "go@1.25.4")
			parts := strings.SplitN(spec, "@", 2)
			if len(parts) != 2 {
				return zerr.Wrap(
					zerr.Wrap(domain.ErrInvalidToolSpec, spec),
					"expected format: package@version",
				)
			}
			version := parts[1]

			// Resolve to commit hash and attribute path
			// Fix: Use the package name from spec (parts[0]) instead of the alias
			// The alias is just the local name for the tool (e.g. "lint"),
			// whereas parts[0] is the actual package name (e.g. "golangci-lint")
			commitHash, attrPath, err := e.resolver.Resolve(groupCtx, parts[0], version)
			if err != nil {
				return zerr.Wrap(err, "failed to resolve tool")
			}

			// Group packages by commit hash
			// We use the attribute path returned by the resolver (e.g., "go_1_22")
			// instead of the alias/package name derived from the spec.
			mu.Lock()
			commitToPackages[commitHash] = append(commitToPackages[commitHash], attrPath)
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return commitToPackages, nil
}
