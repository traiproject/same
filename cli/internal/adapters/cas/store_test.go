package cas_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/cas"
	"go.trai.ch/same/internal/core/domain"
)

func TestStore_PutGet(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store, err := cas.NewStoreWithPath(tmpDir)
	require.NoError(t, err)

	info := domain.BuildInfo{
		TaskName:   "task-1",
		InputHash:  "abc",
		OutputHash: "def",
		Timestamp:  time.Now().Truncate(time.Second), // Truncate because JSON unmarshal might lose precision
	}

	t.Run("put and get", func(t *testing.T) {
		t.Parallel()
		err := store.Put(info)
		require.NoError(t, err)

		got, err := store.Get("task-1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, info, *got)
	})

	t.Run("get missing", func(t *testing.T) {
		t.Parallel()
		got, err := store.Get("missing-task")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("get corrupt", func(t *testing.T) {
		t.Parallel()

		// Use a separate store for corruption test to avoid side effects
		tmpDir2 := t.TempDir()
		store2, err := cas.NewStoreWithPath(tmpDir2)
		require.NoError(t, err)

		info2 := domain.BuildInfo{TaskName: "task-2"}
		err = store2.Put(info2)
		require.NoError(t, err)

		// Corrupt the file. We find it by listing the directory.
		entries, err := os.ReadDir(tmpDir2)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		filename := entries[0].Name()
		//nolint:gosec // 0644 is fine for test
		err = os.WriteFile(tmpDir2+"/"+filename, []byte("{ invalid json"), 0o600)
		require.NoError(t, err)

		_, err = store2.Get("task-2")
		require.Error(t, err)
		assert.ErrorContains(t, err, domain.ErrStoreUnmarshalFailed.Error())
	})
}
