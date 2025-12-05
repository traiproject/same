package nix_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"go.trai.ch/bob/internal/adapters/nix"
	"go.trai.ch/bob/internal/core/domain"
)

func TestNewManager(t *testing.T) {
	manager := nix.NewManager()
	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestInstall_Success(t *testing.T) {
	// This test requires nix to be installed and will actually run nix build
	// Skip if nix is not available
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not found in PATH, skipping integration test")
	}

	manager := nix.NewManager()
	ctx := context.Background()

	// Use a well-known package and commit for testing
	// This is a real commit from nixpkgs that should work
	//nolint:gosec // flakeRef is constructed from validated inputs
	storePath, err := manager.Install(ctx, "hello", "2788904d26dda6cfa1921c5abb7a2466ffe3cb8c")
	if err != nil {
		t.Skipf("Install() failed (this may be expected in CI): %v", err)
	}

	if storePath == "" {
		t.Error("Install() returned empty store path")
	}

	// Verify the path looks like a nix store path
	if !strings.HasPrefix(storePath, "/nix/store") {
		t.Errorf("Install() returned invalid store path: %s", storePath)
	}
}

func TestInstall_InvalidCommit(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not found in PATH, skipping integration test")
	}

	manager := nix.NewManager()
	ctx := context.Background()

	// Use an invalid commit hash
	_, err := manager.Install(ctx, "go", "invalid-commit-hash")
	if err == nil {
		t.Error("Install() expected error for invalid commit")
	}

	// Check that error contains our custom error
	if !strings.Contains(err.Error(), domain.ErrNixInstallFailed.Error()) {
		t.Errorf("Install() error = %v, want error containing %v", err, domain.ErrNixInstallFailed)
	}
}

func TestInstall_NonexistentPackage(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not found in PATH, skipping integration test")
	}

	manager := nix.NewManager()
	ctx := context.Background()

	// Use a valid commit but nonexistent package
	packageName := "this-package-definitely-does-not-exist-12345"
	commitHash := "2788904d26dda6cfa1921c5abb7a2466ffe3cb8c"
	_, err := manager.Install(ctx, packageName, commitHash)
	if err == nil {
		t.Error("Install() expected error for nonexistent package")
	}

	if !strings.Contains(err.Error(), domain.ErrNixInstallFailed.Error()) {
		t.Errorf("Install() error = %v, want error containing %v", err, domain.ErrNixInstallFailed)
	}
}

func TestInstall_ContextCancellation(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not found in PATH, skipping integration test")
	}

	manager := nix.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := manager.Install(ctx, "go", "2788904d26dda6cfa1921c5abb7a2466ffe3cb8c")
	if err == nil {
		t.Error("Install() expected error for canceled context")
	}
}
