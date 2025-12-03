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
	//   bob.work.yaml (workspace: ["packages/*"])
	//   bob.yaml (project: root) -- part of workspace via glob? No, workspace glob usually doesn't include root unless specified.
	//   packages/
	//     pkg-a/
	//       bob.yaml
	//       src/ (cwd for test)
	tmpDir := t.TempDir()

	// Root workspace config
	rootWorkContent := `
version: "1"
workspace: ["packages/*", "."]
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.work.yaml"), []byte(rootWorkContent), 0o600)
	require.NoError(t, err)

	// Root project config
	rootContent := `
version: "1"
project: "root"
tasks:
  root-task:
    cmd: ["echo root"]
`
	err = os.WriteFile(filepath.Join(tmpDir, "bob.yaml"), []byte(rootContent), 0o600)
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
	//   bob.yaml (no workspace, no bob.work.yaml)
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
	// And there is no bob.work.yaml
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

func TestLoad_WorkspaceOverride(t *testing.T) {
	// Structure:
	// root/
	//   bob.work.yaml (workspace: ["packages/*"])
	//   packages/
	//     pkg-a/
	//       bob.yaml
	//       src/ (cwd for test)
	// In this case, even though we are inside pkg-a (which has a bob.yaml),
	// the bubble up should find bob.work.yaml at root and prefer it.
	tmpDir := t.TempDir()

	// Root workspace config
	rootWorkContent := `
version: "1"
workspace: ["packages/*"]
`
	err := os.WriteFile(filepath.Join(tmpDir, "bob.work.yaml"), []byte(rootWorkContent), 0o600)
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

	// Create a subdirectory in pkg-a
	srcDir := filepath.Join(pkgADir, "src")
	err = os.MkdirAll(srcDir, 0o750)
	require.NoError(t, err)

	// Test: Load from deep inside pkg-a, should find root workspace
	loader := &config.FileConfigLoader{Filename: "bob.yaml"}
	g, err := loader.Load(srcDir)
	require.NoError(t, err)

	require.NoError(t, g.Validate())

	tasks := make(map[string]bool)
	for task := range g.Walk() {
		tasks[task.Name.String()] = true
	}
	assert.Contains(t, tasks, "pkg-a:pkg-a-task")
	// If root had a project file included in workspace, we'd check that too,
	// but here we just want to ensure we didn't just load pkg-a as standalone.
	// Hard to tell difference unless we check if other members are loaded.
	// Let's add another member.

	pkgBDir := filepath.Join(packagesDir, "pkg-b")
	err = os.MkdirAll(pkgBDir, 0o750)
	require.NoError(t, err)
	pkgBContent := `
version: "1"
project: "pkg-b"
tasks:
  pkg-b-task:
    cmd: ["echo pkg-b"]
`
	err = os.WriteFile(filepath.Join(pkgBDir, "bob.yaml"), []byte(pkgBContent), 0o600)
	require.NoError(t, err)

	// Reload
	g, err = loader.Load(srcDir)
	require.NoError(t, err)
	require.NoError(t, g.Validate())

	tasks = make(map[string]bool)
	for task := range g.Walk() {
		tasks[task.Name.String()] = true
	}
	assert.Contains(t, tasks, "pkg-a:pkg-a-task")
	assert.Contains(t, tasks, "pkg-b:pkg-b-task") // This proves we loaded via workspace
}
