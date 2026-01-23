package cas_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.trai.ch/same/internal/adapters/cas"
	"go.trai.ch/same/internal/core/domain"
)

func TestNewStore(t *testing.T) {
	// NewStore uses a hardcoded path ".same/store"
	// We need to test in a temp directory context
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}

	tmpDir := t.TempDir()
	if cdErr := os.Chdir(tmpDir); cdErr != nil {
		t.Fatalf("Chdir failed: %v", cdErr)
	}
	defer func() {
		if chErr := os.Chdir(originalWd); chErr != nil {
			t.Errorf("Failed to restore working directory: %v", chErr)
		}
	}()

	store, err := cas.NewStore()
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("NewStore returned nil store")
	}

	// Verify the directory was created
	// .same/store is the default path
	expectedPath := filepath.Join(tmpDir, domain.DefaultStorePath())
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Errorf("NewStore did not create directory at %s", expectedPath)
	}
}

func TestStore_PutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "same_state")

	store, err := cas.NewStoreWithPath(storePath)
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
		return
	}

	if got.TaskName != info.TaskName {
		t.Errorf("expected TaskName %q, got %q", info.TaskName, got.TaskName)
	}
}

func TestStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "same_state")

	// 1. Create store and save data
	store1, err := cas.NewStoreWithPath(storePath)
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
	store2, err2 := cas.NewStoreWithPath(storePath)
	if err2 != nil {
		t.Fatalf("NewStore 2 failed: %v", err2)
	}

	got, err3 := store2.Get("task2")
	if err3 != nil {
		t.Fatalf("Get failed: %v", err3)
	}
	if got == nil {
		t.Fatal("Get returned nil")
		return
	}
	if got.InputHash != "xyz" {
		t.Errorf("expected InputHash %q, got %q", "xyz", got.InputHash)
	}
}

func TestStore_OmitZero(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "same_state")

	store, err := cas.NewStoreWithPath(storePath)
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

func TestNewStore_Error(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file where the directory should be
	filePath := filepath.Join(tmpDir, "file_blocking_dir")
	if err := os.WriteFile(filePath, []byte("block"), domain.PrivateFilePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := cas.NewStoreWithPath(filePath)
	if err == nil {
		t.Fatal("NewStore should have failed when path is a file")
	}
}

func TestGet_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "same_state")
	store, err := cas.NewStoreWithPath(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Manually create a task file with no read permissions
	taskName := "task_read_error"
	hash := sha256.Sum256([]byte(taskName))
	hexHash := hex.EncodeToString(hash[:])
	taskFile := filepath.Join(storePath, hexHash+".json")

	// Write only
	//nolint:gosec // Intentionally creating a file with weird permissions for testing
	if err = os.WriteFile(taskFile, []byte("{}"), 0o200); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err = store.Get(taskName)
	if err == nil {
		t.Fatal("Get should have failed due to read permissions")
	}
}

func TestGet_UnmarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "same_state")
	store, err := cas.NewStoreWithPath(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Manually create a task file with invalid JSON
	taskName := "task_invalid_json"
	hash := sha256.Sum256([]byte(taskName))
	hexHash := hex.EncodeToString(hash[:])
	taskFile := filepath.Join(storePath, hexHash+".json")

	if err = os.WriteFile(taskFile, []byte("{ invalid json"), domain.PrivateFilePerm); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err = store.Get(taskName)
	if err == nil {
		t.Fatal("Get should have failed due to invalid JSON")
	}
}

func TestPut_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "same_state")
	store, err := cas.NewStoreWithPath(storePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Remove write permissions from the directory
	//nolint:gosec // Intentionally restricting permissions for testing
	if err = os.Chmod(storePath, 0o500); err != nil { // Read/Execute only
		t.Fatalf("Chmod failed: %v", err)
	}
	defer func() {
		//nolint:gosec // Restoring permissions to standard value
		if chmodErr := os.Chmod(storePath, domain.DirPerm); chmodErr != nil {
			t.Errorf("Failed to restore permissions: %v", chmodErr)
		}
	}() // Restore permissions for cleanup

	info := domain.BuildInfo{
		TaskName: "task_write_error",
	}

	err = store.Put(info)
	if err == nil {
		t.Fatal("Put should have failed due to directory permissions")
	}
}
