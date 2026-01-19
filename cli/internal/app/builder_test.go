package app_test

import (
	"context"
	"os"
	"testing"

	"github.com/grindlemire/graft"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/app"
	_ "go.trai.ch/same/internal/wiring" // Register providers
)

func TestAppWiring(t *testing.T) {
	// Use a temporary directory for the test
	cwd, err := os.Getwd()
	require.NoError(t, err)

	defer func() {
		errChdir := os.Chdir(cwd)
		require.NoError(t, errChdir)
	}()

	tmpDir := t.TempDir()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Verify that the application graph can be constructed
	components, _, err := graft.ExecuteFor[*app.Components](context.Background())
	require.NoError(t, err)
	require.NotNil(t, components)
	require.NotNil(t, components.App)
	require.NotNil(t, components.Logger)
}
