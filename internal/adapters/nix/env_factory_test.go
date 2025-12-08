package nix_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.trai.ch/bob/internal/adapters/nix"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestNewEnvFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	factory := nix.NewEnvFactory(resolver, manager, "/tmp/cache")
	if factory == nil {
		t.Fatal("NewEnvFactory() returned nil")
	}
}

func TestGetEnvironment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	// Mock resolver to return commit hashes
	resolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.25.4").
		Return("abc123", nil)

	factory := nix.NewEnvFactory(resolver, manager, "/tmp/cache")
	ctx := context.Background()

	tools := map[string]string{
		"go": "go@1.25.4",
	}

	env, err := factory.GetEnvironment(ctx, tools)
	if err != nil {
		t.Fatalf("GetEnvironment() error = %v", err)
	}

	if env == nil {
		t.Error("GetEnvironment() returned nil environment")
	}
}

func TestGetEnvironment_InvalidSpec(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	factory := nix.NewEnvFactory(resolver, manager, "/tmp/cache")
	ctx := context.Background()

	tools := map[string]string{
		"go": "invalid-spec-without-at",
	}

	_, err := factory.GetEnvironment(ctx, tools)
	if err == nil {
		t.Error("GetEnvironment() expected error for invalid spec")
	}

	if !strings.Contains(err.Error(), "invalid tool spec format") {
		t.Errorf("GetEnvironment() error = %v, want error containing 'invalid tool spec format'", err)
	}
}

func TestGetEnvironment_ResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	resolver := mocks.NewMockDependencyResolver(ctrl)
	manager := mocks.NewMockPackageManager(ctrl)

	// Mock resolver to return error
	resolver.EXPECT().
		Resolve(gomock.Any(), "go", "1.25.4").
		Return("", fmt.Errorf("resolver error"))

	factory := nix.NewEnvFactory(resolver, manager, "/tmp/cache")
	ctx := context.Background()

	tools := map[string]string{
		"go": "go@1.25.4",
	}

	_, err := factory.GetEnvironment(ctx, tools)
	if err == nil {
		t.Error("GetEnvironment() expected error when resolver fails")
	}
}
