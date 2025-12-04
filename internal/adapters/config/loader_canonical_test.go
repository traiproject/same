package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.trai.ch/bob/internal/adapters/config"
	"go.trai.ch/bob/internal/core/domain"
	"go.trai.ch/bob/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestLoad_Canonicalization(t *testing.T) {
	content := `
version: "1"
tasks:
  build:
    input: ["b", "a", "a", "c"]
    cmd: ["echo"]
    target: ["y", "x", "x", "z"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bob.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctrl := gomock.NewController(t)
	loader := &config.Loader{Logger: mocks.NewMockLogger(ctrl)}
	g, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := g.Validate(); err != nil {
		t.Fatalf("graph validation failed: %v", err)
	}

	var taskName string
	for task := range g.Walk() {
		taskName = task.Name.String()
		if taskName == "build" {
			// Check Inputs
			expectedInputs := []string{"a", "b", "c"}
			checkSlice(t, "Inputs", expectedInputs, task.Inputs)

			// Check Outputs
			expectedOutputs := []string{"x", "y", "z"}
			checkSlice(t, "Outputs", expectedOutputs, task.Outputs)
		}
	}
	if taskName == "" {
		t.Fatal("task 'build' not found")
	}
}

func checkSlice(t *testing.T, name string, expected []string, actual []domain.InternedString) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Errorf("expected %d %s, got %d", len(expected), name, len(actual))
		return
	}
	for i, val := range actual {
		if val.String() != expected[i] {
			t.Errorf("expected %s %d to be %s, got %s", name, i, expected[i], val.String())
		}
	}
}
