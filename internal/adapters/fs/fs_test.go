package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.trai.ch/bob/internal/adapters/fs"
	"go.trai.ch/bob/internal/core/domain"
)

func TestWalker_WalkFiles(t *testing.T) { //nolint:cyclop // Test complexity is acceptable
	// Create temp directory structure
	// tmp/
	//   .git/
	//     config
	//   ignored/
	//     file
	//   src/
	//     main.go
	//   README.md

	tmpDir, err := os.MkdirTemp("", "walker_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // Best effort cleanup in test

	// Create .git directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o750); err != nil { //nolint:gosec // Test directory permissions
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".git", "config"), []byte("git config"), 0o600); err != nil { //nolint:gosec // Test file permissions
		t.Fatal(err)
	}

	// Create ignored directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "ignored"), 0o750); err != nil { //nolint:gosec // Test directory permissions
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ignored", "file"), []byte("ignored content"), 0o600); err != nil { //nolint:gosec // Test file permissions
		t.Fatal(err)
	}

	// Create src directory
	if err := os.MkdirAll(filepath.Join(tmpDir, "src"), 0o750); err != nil { //nolint:gosec // Test directory permissions
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0o600); err != nil { //nolint:gosec // Test file permissions
		t.Fatal(err)
	}

	// Create README.md
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Readme"), 0o600); err != nil { //nolint:gosec // Test file permissions
		t.Fatal(err)
	}

	walker := fs.NewWalker()
	ignores := []string{"ignored"}

	files := make(map[string]bool)
	for path := range walker.WalkFiles(tmpDir, ignores) {
		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			t.Fatal(err)
		}
		files[rel] = true
	}

	// Assertions
	if files[".git/config"] {
		t.Error("expected .git/config to be skipped")
	}
	if files["ignored/file"] {
		t.Error("expected ignored/file to be skipped")
	}
	if !files["src/main.go"] {
		t.Error("expected src/main.go to be found")
	}
	if !files["README.md"] {
		t.Error("expected README.md to be found")
	}
}

func TestHasher_ComputeFileHash(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "hasher_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name()) //nolint:errcheck // Best effort cleanup in test

	content := []byte("hello world")
	n, writeErr := tmpFile.Write(content)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	_ = n
	_ = tmpFile.Close()

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	hash1, err := hasher.ComputeFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("ComputeFileHash failed: %v", err)
	}

	if hash1 == 0 {
		t.Error("expected non-zero hash")
	}

	// Verify determinism
	hash2, err := hasher.ComputeFileHash(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if hash1 != hash2 {
		t.Error("expected deterministic hash")
	}
}

func TestHasher_ComputeInputHash(t *testing.T) { //nolint:cyclop // Test complexity is acceptable
	tmpDir, err := os.MkdirTemp("", "input_hash_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // Best effort cleanup in test

	// Create input file
	inputFile := filepath.Join(tmpDir, "input.txt")
	if writeErr := os.WriteFile(inputFile, []byte("input content"), 0o600); writeErr != nil { //nolint:gosec // Test file permissions
		t.Fatal(writeErr)
	}

	walker := fs.NewWalker()
	hasher := fs.NewHasher(walker)

	task := &domain.Task{
		Name:   domain.NewInternedString("task1"),
		Inputs: []domain.InternedString{domain.NewInternedString("input.txt")},
	}
	env := map[string]string{"KEY": "VALUE"}

	hash1, err := hasher.ComputeInputHash(task, env, tmpDir)
	if err != nil {
		t.Fatalf("ComputeInputHash failed: %v", err)
	}

	// 1. Verify hash changes with task definition
	task2 := &domain.Task{
		Name:   domain.NewInternedString("task2"),
		Inputs: []domain.InternedString{domain.NewInternedString("input.txt")},
	}
	hash2, err := hasher.ComputeInputHash(task2, env, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash2 {
		t.Error("expected hash to change when task name changes")
	}

	// 2. Verify hash changes with env
	env2 := map[string]string{"KEY": "VALUE2"}
	hash3, err := hasher.ComputeInputHash(task, env2, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash3 {
		t.Error("expected hash to change when env changes")
	}

	// 3. Verify hash changes with file content
	if writeErr := os.WriteFile(inputFile, []byte("modified content"), 0o600); writeErr != nil { //nolint:gosec // Test file permissions
		t.Fatal(writeErr)
	}
	hash4, err := hasher.ComputeInputHash(task, env, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash4 {
		t.Error("expected hash to change when file content changes")
	}
}
