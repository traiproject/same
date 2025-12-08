package nix

import (
	"context"
	"fmt"
	"strings"

	"go.trai.ch/bob/internal/core/ports"
	"go.trai.ch/zerr"
)

// EnvFactory implements ports.EnvironmentFactory using Nix.
type EnvFactory struct {
	resolver ports.DependencyResolver
	manager  ports.PackageManager
	cacheDir string
}

// NewEnvFactory creates a new EnvironmentFactory backed by Nix.
func NewEnvFactory(
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
	// Resolve all tools to commit hashes
	commitToPackages := make(map[string][]string)

	for alias, spec := range tools {
		// Parse spec to get package name and version
		// Spec format: "package@version" (e.g., "go@1.25.4")
		parts := strings.SplitN(spec, "@", 2)
		if len(parts) != 2 {
			return nil, zerr.Wrap(
				fmt.Errorf("invalid tool spec format: %s", spec),
				"expected format: package@version",
			)
		}
		packageName := parts[0]
		version := parts[1]

		// Resolve to commit hash
		commitHash, err := e.resolver.Resolve(ctx, alias, version)
		if err != nil {
			return nil, zerr.Wrap(err, "failed to resolve tool")
		}

		// Group packages by commit hash
		commitToPackages[commitHash] = append(commitToPackages[commitHash], packageName)
	}

	// Generate Nix expression
	system := getCurrentSystem()
	nixExpr := e.generateNixExpr(system, commitToPackages)

	// For now, return empty environment
	// TODO: Write nixExpr to file, build it, extract environment
	_ = nixExpr

	return []string{}, nil
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
	builder.WriteString("  buildInputs = [\n")

	// Add all packages
	for commitHash, packages := range commits {
		idx := commitToIdx[commitHash]
		for _, pkg := range packages {
			builder.WriteString(fmt.Sprintf("    pkgs_%d.%s\n", idx, pkg))
		}
	}

	builder.WriteString("  ];\n")
	builder.WriteString("}\n")

	return builder.String()
}
