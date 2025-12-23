package app_test

import (
	"context"
	"testing"

	"github.com/grindlemire/graft"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/app"
	_ "go.trai.ch/bob/internal/wiring" // Register providers
)

func TestAppWiring(t *testing.T) {
	// Verify that the application graph can be constructed
	components, _, err := graft.ExecuteFor[*app.Components](context.Background())
	require.NoError(t, err)
	require.NotNil(t, components)
	require.NotNil(t, components.App)
	require.NotNil(t, components.Logger)
}
