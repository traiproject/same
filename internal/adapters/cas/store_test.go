package cas_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.trai.ch/bob/internal/adapters/cas"
	"go.trai.ch/bob/internal/core/domain"
)

func TestStore_PutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "bob_state")

	store, err := cas.NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	info := domain.BuildInfo{
		TaskName:   "task1",
		InputHash:  "abc",
		OutputHash: "def",
		Timestamp:  time.Now(),
	}

	err2 := store.Put(info)
	if err2 != nil {
		t.Fatalf("Put failed: %v", err2)
	}

	got, err := store.Get("task1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}

	if got.TaskName != info.TaskName {
		t.Errorf("expected TaskName %q, got %q", info.TaskName, got.TaskName)
	}
}

func TestStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "bob_state")

	// 1. Create store and save data
	store1, err := cas.NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore 1 failed: %v", err)
	}

	info := domain.BuildInfo{
		TaskName:  "task2",
		InputHash: "xyz",
	}
	if err := store1.Put(info); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 2. Create new store instance pointing to same directory
	store2, err2 := cas.NewStore(storePath)
	if err2 != nil {
		t.Fatalf("NewStore 2 failed: %v", err2)
	}

	got, err3 := store2.Get("task2")
	if err3 != nil {
		t.Fatalf("Get failed: %v", err3)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.InputHash != "xyz" {
		t.Errorf("expected InputHash %q, got %q", "xyz", got.InputHash)
	}
}

func TestStore_OmitZero(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "bob_state")

	store, err := cas.NewStore(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create info with zero values for hashes and timestamp
	info := domain.BuildInfo{
		TaskName: "task_zero",
	}

	err2 := store.Put(info)
	if err2 != nil {
		t.Fatalf("Put failed: %v", err2)
	}

	// Read the file content directly
	hash := sha256.Sum256([]byte("task_zero"))
	hexHash := hex.EncodeToString(hash[:])
	taskFile := filepath.Join(storePath, hexHash+".json")

	//nolint:gosec // Test file with controlled path
	content, err := os.ReadFile(taskFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	jsonStr := string(content)
	t.Logf("JSON content: %s", jsonStr)

	// Verify fields are omitted
	if strings.Contains(jsonStr, "input_hash") {
		t.Error("JSON should not contain 'input_hash' for zero value")
	}
	if strings.Contains(jsonStr, "output_hash") {
		t.Error("JSON should not contain 'output_hash' for zero value")
	}
	if strings.Contains(jsonStr, "timestamp") {
		t.Error("JSON should not contain 'timestamp' for zero value")
	}
	// TaskName should be present
	if !strings.Contains(jsonStr, "task_name") {
		t.Error("JSON should contain 'task_name'")
	}
}
