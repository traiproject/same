package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/bob/internal/adapters/config"
)

func TestLoad_WorkspaceDiscovery(t *testing.T) {
	// Structure:
	// root/
	//   bob.yaml (workspace: ["packages/*"])
	//   packages/
	//     pkg-a/
	//       bob.yaml
	//       src/ (cwd for test)
	//     pkg-b/
	//       bob.yaml
	tmpDir := t.TempDir()

	// Root config
	rootContent := `
version: "1"
project: "root"
workspace: ["packages/*"]
tasks:
  root-task:
    cmd: ["echo root"]
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.yaml"), []byte(rootContent), 0o600)
	require.NoError(t, err)

	// Packages directory
	packagesDir := filepath.Join(tmpDir, "packages")
	err = os.MkdirAll(packagesDir, 0o750)
	require.NoError(t, err)

	// Package A
	pkgADir := filepath.Join(packagesDir, "pkg-a")
	err = os.MkdirAll(pkgADir, 0o750)
	require.NoError(t, err)
	pkgAContent := `
version: "1"
project: "pkg-a"
tasks:
  pkg-a-task:
    cmd: ["echo pkg-a"]
`
	err = os.WriteFile(filepath.Join(pkgADir, "bob.yaml"), []byte(pkgAContent), 0o600)
	require.NoError(t, err)

	// Create a subdirectory in pkg-a to simulate running from deep inside
	srcDir := filepath.Join(pkgADir, "src")
	err = os.MkdirAll(srcDir, 0o750)
	require.NoError(t, err)

	// Test 1: Load from deep inside pkg-a, should find root workspace
	loader := &config.FileConfigLoader{Filename: "bob.yaml"}
	g, err := loader.Load(srcDir)
	require.NoError(t, err)

	// Validate to populate execution order
	require.NoError(t, g.Validate())

	// Verify we loaded the whole workspace
	tasks := make(map[string]bool)
	for task := range g.Walk() {
		tasks[task.Name.String()] = true
	}
	assert.Contains(t, tasks, "root:root-task")
	assert.Contains(t, tasks, "pkg-a:pkg-a-task")
}

func TestLoad_StandaloneDiscovery(t *testing.T) {
	// Structure:
	// root/
	//   bob.yaml (no workspace)
	//   src/ (cwd for test)
	tmpDir := t.TempDir()

	content := `
version: "1"
project: "standalone"
tasks:
  build:
    cmd: ["echo build"]
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.yaml"), []byte(content), 0o600)
	require.NoError(t, err)

	srcDir := filepath.Join(tmpDir, "src")
	err = os.MkdirAll(srcDir, 0o750)
	require.NoError(t, err)

	loader := &config.FileConfigLoader{Filename: "bob.yaml"}
	g, err := loader.Load(srcDir)
	require.NoError(t, err)

	// Validate to populate execution order
	require.NoError(t, g.Validate())

	tasks := make(map[string]bool)
	for task := range g.Walk() {
		tasks[task.Name.String()] = true
	}
	assert.Contains(t, tasks, "standalone:build")
}

func TestLoad_NestedStandalone(t *testing.T) {
	// Structure:
	// root/
	//   bob.yaml (project: root)
	//   nested/
	//     bob.yaml (project: nested)
	//     src/ (cwd for test)
	// Should pick up 'nested' as root because it's the nearest config and 'root' is not a workspace
	tmpDir := t.TempDir()

	rootContent := `
version: "1"
project: "root"
tasks: {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.yaml"), []byte(rootContent), 0o600)
	require.NoError(t, err)

	nestedDir := filepath.Join(tmpDir, "nested")
	err = os.MkdirAll(nestedDir, 0o750)
	require.NoError(t, err)

	nestedContent := `
version: "1"
project: "nested"
tasks:
  nested-task:
    cmd: ["echo nested"]
`
	err = os.WriteFile(filepath.Join(nestedDir, "bob.yaml"), []byte(nestedContent), 0o600)
	require.NoError(t, err)

	srcDir := filepath.Join(nestedDir, "src")
	err = os.MkdirAll(srcDir, 0o750)
	require.NoError(t, err)

	loader := &config.FileConfigLoader{Filename: "bob.yaml"}
	g, err := loader.Load(srcDir)
	require.NoError(t, err)

	// Validate to populate execution order
	require.NoError(t, g.Validate())

	tasks := make(map[string]bool)
	for task := range g.Walk() {
		tasks[task.Name.String()] = true
	}
	assert.Contains(t, tasks, "nested:nested-task")
	assert.NotContains(t, tasks, "root:root-task") // Should NOT load root
}
