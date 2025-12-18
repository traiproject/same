package app_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/app"
)

func TestNewApp_Success(t *testing.T) {
	// Call NewApp - it now uses internal defaults for paths
	components, err := app.NewApp()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify components are initialized
	require.NotNil(t, components)
	require.NotNil(t, components.App)
	require.NotNil(t, components.Logger)
}
