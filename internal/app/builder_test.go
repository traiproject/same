package app_test

import (
	"testing"

	"go.trai.ch/bob/internal/app"
)

func TestNewApp_Success(t *testing.T) {
	// Call NewApp - it now uses internal defaults for paths
	components, err := app.NewApp()
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
