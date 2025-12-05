//nolint:testpackage // Testing internal parsing logic
package nix

import (
	"strings"
	"testing"

	"go.trai.ch/bob/internal/core/domain"
)

func TestParseBuildResults_Success(t *testing.T) {
	output := []byte(`[
		{
			"drvPath": "/nix/store/drv",
			"outputs": {
				"out": "/nix/store/out-path"
			}
		}
	]`)

	path, err := parseBuildResults(output, "tool", "commit")
	if err != nil {
		t.Fatalf("parseBuildResults() error = %v", err)
	}
	if path != "/nix/store/out-path" {
		t.Errorf("path = %v, want /nix/store/out-path", path)
	}
}

func TestParseBuildResults_InvalidJSON(t *testing.T) {
	output := []byte(`invalid json`)
	_, err := parseBuildResults(output, "tool", "commit")
	if err == nil {
		t.Error("parseBuildResults() expected error for invalid JSON")
	}
	// Error checking depends on how wrap works, assuming it preserves or wraps standard errors
	if !strings.Contains(err.Error(), "failed to parse nix build JSON output") {
		t.Errorf("error = %v, want error containing 'failed to parse nix build JSON output'", err)
	}
}

func TestParseBuildResults_EmptyResults(t *testing.T) {
	output := []byte(`[]`)
	_, err := parseBuildResults(output, "tool", "commit")
	if err == nil {
		t.Error("parseBuildResults() expected error for empty results")
	}
	if !strings.Contains(err.Error(), domain.ErrNixInstallFailed.Error()) {
		t.Errorf("error = %v, want error containing %v", err, domain.ErrNixInstallFailed)
	}
	if !strings.Contains(err.Error(), "empty build results from nix build") {
		t.Errorf("error = %v, want error containing 'empty build results from nix build'", err)
	}
}

func TestParseBuildResults_MissingOut(t *testing.T) {
	output := []byte(`[
		{
			"drvPath": "/nix/store/drv",
			"outputs": {
				"dev": "/nix/store/dev-path"
			}
		}
	]`)
	_, err := parseBuildResults(output, "tool", "commit")
	if err == nil {
		t.Error("parseBuildResults() expected error for missing out")
	}
	if !strings.Contains(err.Error(), domain.ErrNixInstallFailed.Error()) {
		t.Errorf("error = %v, want error containing %v", err, domain.ErrNixInstallFailed)
	}
	if !strings.Contains(err.Error(), "no 'out' output found in build results") {
		t.Errorf("error = %v, want error containing \"no 'out' output found in build results\"", err)
	}
}
