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

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
	"golang.org/x/sync/errgroup"
)

// EnvFactory implements ports.EnvironmentFactory using Nix.
type EnvFactory struct {
	resolver ports.DependencyResolver
	manager  ports.PackageManager
	cacheDir string
}

// NewEnvFactoryWithCache creates a new EnvironmentFactory backed by Nix with a specific cache directory.
func NewEnvFactoryWithCache(
	resolver ports.DependencyResolver,
	manager ports.PackageManager,
	cacheDir string,
) *EnvFactory {
	return &EnvFactory{
		resolver: resolver,
		manager:  manager,
		cacheDir: cacheDir,
	}
}

// GetEnvironment constructs a hermetic environment from a set of tools.
// The tools map contains alias->spec pairs (e.g., "go" -> "go@1.25.4").
// Returns environment variables as "KEY=VALUE" strings suitable for process execution.
func (e *EnvFactory) GetEnvironment(ctx context.Context, tools map[string]string) ([]string, error) {
	// Step A: Resolve all tools to commit hashes
	commitToPackages := make(map[string][]string)
	var mu sync.Mutex

	g, groupCtx := errgroup.WithContext(ctx)
	// Use number of CPUs as concurrency limit, matching scheduler default
	g.SetLimit(runtime.NumCPU())

	for alias, spec := range tools {
		alias, spec := alias, spec // Capture loop variables
		g.Go(func() error {
			// Parse spec to get package name and version
			// Spec format: "package@version" (e.g., "go@1.25.4")
			parts := strings.SplitN(spec, "@", 2)
			if len(parts) != 2 {
				return zerr.Wrap(
					fmt.Errorf("invalid tool spec format: %s", spec),
					"expected format: package@version",
				)
			}
			version := parts[1]

			// Resolve to commit hash and attribute path
			commitHash, attrPath, err := e.resolver.Resolve(groupCtx, alias, version)
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

	// Step B: Create deterministic hash of toolset for cache key
	envID := domain.GenerateEnvID(tools)

	// Step C: Check cache
	cachePath := filepath.Join(e.cacheDir, "environments", envID+".json")
	if cachedEnv, err := LoadEnvFromCache(cachePath); err == nil {
		return cachedEnv, nil
	}

	// Step D: Generate and execute Nix expression
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
	// Step E: Persist to cache

	// Enforce local toolchain for Go to prevent auto-downloading newer versions
	// based on go.mod directive.
	env = append(env, "GOTOOLCHAIN=local")
	slices.Sort(env) // Re-sort after appending

	if err := SaveEnvToCache(cachePath, env); err != nil {
		// Log warning but don't fail - cache write is not critical
		_ = err
	}

	return env, nil
}

// generateNixExpr generates a Nix expression from a commit-to-packages mapping.
func (e *EnvFactory) generateNixExpr(system string, commits map[string][]string) string {
	var builder strings.Builder

	// Start let block
	builder.WriteString("let\n")
	builder.WriteString(fmt.Sprintf("system = %q;\n", system))

	// Generate flake and pkgs variables for each commit
	commitIdx := 0
	commitToIdx := make(map[string]int)

	for commitHash := range commits {
		builder.WriteString(fmt.Sprintf("flake_%d = builtins.getFlake \"github:NixOS/nixpkgs/%s\";\n",
			commitIdx, commitHash))
		builder.WriteString(fmt.Sprintf("pkgs_%d = flake_%d.legacyPackages.${system};\n",
			commitIdx, commitIdx))
		commitToIdx[commitHash] = commitIdx
		commitIdx++
	}

	// Start mkShell block
	builder.WriteString("in\n")

	// Use the first pkgs for mkShell (arbitrary choice, all should have mkShell)
	firstIdx := 0
	if len(commitToIdx) > 0 {
		// Get any index from the map
		for _, idx := range commitToIdx {
			firstIdx = idx
			break
		}
	}

	builder.WriteString(fmt.Sprintf("pkgs_%d.mkShell {\n", firstIdx))
	builder.WriteString("buildInputs = [\n")

	// Add all packages
	for commitHash, packages := range commits {
		idx := commitToIdx[commitHash]
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
	tmpFile, err := os.CreateTemp("", "bob-env-*.nix")
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
			return nil, fmt.Errorf("cache miss")
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
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return zerr.Wrap(err, "failed to create cache directory")
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return zerr.Wrap(err, "failed to marshal environment")
	}

	//nolint:gosec // Path is constructed from trusted cache directory
	if err := os.WriteFile(path, data, filePerm); err != nil {
		return zerr.Wrap(err, "failed to write cache file")
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
	// Always include these
	include := []string{
		"PATH",
		"GOROOT",
		"GOPATH",
		"GOCACHE",
		"CC",
		"CXX",
		"LD",
		"AR",
		"CFLAGS",
		"CXXFLAGS",
		"LDFLAGS",
		"PKG_CONFIG_PATH",
		"NIX_",
	}

	for _, prefix := range include {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	// Exclude interactive shell variables
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
	}

	for _, excluded := range exclude {
		if key == excluded {
			return false
		}
	}

	// Include anything else that looks build-related
	return false
}
