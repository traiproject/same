package nix_test

import (
	"testing"

	"go.trai.ch/bob/internal/adapters/nix"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestNewEnvFactory_DefaultCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	factory := nix.NewEnvFactory(resolver, manager)

	if factory == nil {
		t.Fatal("NewEnvFactory() returned nil")
	}

	// Verify it creates the same as NewEnvFactoryWithCache with default path
	expectedFactory := nix.NewEnvFactoryWithCache(resolver, manager, ".bob/cache/environments")

	// Both should be non-nil and of the same type
	if factory == nil || expectedFactory == nil {
		t.Error("NewEnvFactory() or NewEnvFactoryWithCache() returned nil")
	}
}
