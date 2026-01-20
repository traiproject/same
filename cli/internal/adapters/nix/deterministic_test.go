package nix_test

import (
	"slices"
	"strings"
	"testing"

	"go.trai.ch/same/internal/adapters/nix"
)

func TestGenerateNixExpr_Deterministic(t *testing.T) {
	// Create a commit-to-packages map with multiple entries to trigger map iteration randomness
	commits := map[string][]string{
		"commit_A": {"pkg3", "pkg1", "pkg2"},
		"commit_B": {"pkg5", "pkg4"},
		"commit_C": {"pkg6"},
	}

	factory := nix.NewEnvFactoryWithCache(nil, "/tmp/cache")
	system := "x86_64-darwin"

	// Run multiple times and ensure output is identical
	var firstOutput string
	for i := 0; i < 20; i++ {
		// Use the exported test method
		output := factory.GenerateNixExprForTest(system, commits)
		if i == 0 {
			firstOutput = output
		} else if output != firstOutput {
			t.Fatalf("generateNixExpr is not deterministic on iteration %d\nFirst:\n%s\n\nCurrent:\n%s", i, firstOutput, output)
		}

		// Also verify that the packages for commit_A are sorted (pkg1, pkg2, pkg3)
		// This requires the fix to be implemented, otherwise it might fail on this check even if deterministic.
	}

	// Verify specific ordering expectations (requires knowledge of implementation details like 0, 1, 2 indices)
	// Since we sort keys, "commit_A" should be processed first.
	// If the implementation is sorted:
	// commit_A -> flake_0
	// commit_B -> flake_1
	// commit_C -> flake_2

	if !strings.Contains(firstOutput, "pkgs_0.pkg1") {
		t.Error("Output should contain pkgs_0.pkg1 (sorted package inside sorted commit)")
	}
	// Check for correct package ordering in output
	idx1 := strings.Index(firstOutput, "pkgs_0.pkg1")
	idx2 := strings.Index(firstOutput, "pkgs_0.pkg2")
	idx3 := strings.Index(firstOutput, "pkgs_0.pkg3")

	if idx1 == -1 || idx2 == -1 || idx3 == -1 {
		t.Fatal("Missing expected packages in output")
	}

	// Verify order is strictly increasing
	indices := []int{idx1, idx2, idx3}
	if !slices.IsSorted(indices) {
		t.Error("Packages for commit_A are not sorted alphabetically in output")
	}
}
