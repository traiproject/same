package nix

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/zerr"
)

// Manager implements ports.PackageManager using the Nix CLI.
type Manager struct{}

// NewManager creates a new PackageManager backed by Nix CLI.
func NewManager() *Manager {
	return &Manager{}
}

// Install ensures the tool from the specific commit is available in the Nix store.
// Returns the absolute path to the tool's store path.
func (m *Manager) Install(ctx context.Context, toolName, commitHash string) (string, error) {
	// Construct the flake reference: github:NixOS/nixpkgs/<commitHash>#<toolName>
	flakeRef := fmt.Sprintf("github:NixOS/nixpkgs/%s#%s", commitHash, toolName)

	// Use nix build --json to get the store path in JSON format
	// We use --no-link to avoid creating result symlinks
	//nolint:gosec // flakeRef is constructed from validated inputs
	cmd := exec.CommandContext(ctx, "nix", "build", "--json", "--no-link", flakeRef)

	output, err := cmd.Output()
	if err != nil {
		// Handle exit errors to capture stderr for debugging
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Clean up stderr for the error message
			stderr := strings.TrimSpace(string(exitErr.Stderr))

			nixErr := zerr.Wrap(exitErr, domain.ErrNixInstallFailed.Error())
			nixErr = zerr.With(nixErr, "tool", toolName)
			nixErr = zerr.With(nixErr, "commit", commitHash)
			return "", zerr.With(nixErr, "stderr", stderr)
		}

		// General error handling
		nixErr := zerr.Wrap(err, domain.ErrNixInstallFailed.Error())
		nixErr = zerr.With(nixErr, "tool", toolName)
		return "", zerr.With(nixErr, "commit", commitHash)
	}

	// Parse json output.
	var results buildResults
	if err := json.Unmarshal(output, &results); err != nil {
		parseErr := zerr.Wrap(err, "failed to parse nix build JSON output")
		parseErr = zerr.With(parseErr, "tool", toolName)
		return "", zerr.With(parseErr, "commit", commitHash)
	}

	if len(results) == 0 {
		emptyErr := zerr.With(domain.ErrNixInstallFailed, "tool", toolName)
		emptyErr = zerr.With(emptyErr, "commit", commitHash)
		return "", zerr.With(emptyErr, "reason", "empty build results from nix build")
	}

	storePath, ok := results[0].Outputs["out"]
	if !ok || storePath == "" {
		outErr := zerr.With(domain.ErrNixInstallFailed, "tool", toolName)
		outErr = zerr.With(outErr, "commit", commitHash)
		return "", zerr.With(outErr, "reason", "no 'out' output found in build results")
	}

	return storePath, nil
}
