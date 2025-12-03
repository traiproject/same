package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.trai.ch/bob/internal/app"
)

func TestNewApp_Success(t *testing.T) {
	// Create a temporary directory for the state store
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")

	// Call NewApp
	components, err := app.NewApp("bob.yaml", stateDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify components are initialized
	if components == nil {
		t.Fatal("Expected components to be non-nil")
	}
	if components.App == nil {
		t.Error("Expected App to be initialized")
	}
	if components.Logger == nil {
		t.Error("Expected Logger to be initialized")
	}
}

func TestNewApp_InvalidStateDir(t *testing.T) {
	// Use a path that cannot be created (file exists where directory should be)
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "file_not_dir")

	// Create a file at the state directory path
	if err := os.WriteFile(invalidPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("Failed to set up test: %v", err)
	}

	// Try to create a state directory where a file exists
	// This should fail when NewStore tries to create subdirectories
	_, err := app.NewApp("bob.yaml", invalidPath)

	// We expect an error
	if err == nil {
		t.Error("Expected error when state directory cannot be created, got nil")
	}
}

func TestAppComponents_SetConfigPath(t *testing.T) {
	// Create a temporary directory for the state store
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")

	// Initialize app components
	components, err := app.NewApp("bob.yaml", stateDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Set a new config path
	newPath := "custom_bob.yaml"
	components.SetConfigPath(newPath)

	// We can't directly verify the internal state changed,
	// but we can verify the method doesn't panic and returns cleanly
	// The actual functionality is tested through integration tests
}
