package watcher_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/watcher"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/same/internal/core/ports/mocks"
	"go.uber.org/mock/gomock"
)

func TestNewHashCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)
	require.NotNil(t, cache)
}

func TestHashCache_GetInputHash_Unknown(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	result := cache.GetInputHash("nonexistent", "/project", nil)

	assert.Equal(t, ports.HashUnknown, result.State)
	assert.Empty(t, result.Hash)
}

func TestHashCache_GetInputHash_Ready(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return([]string{"/project/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	result := cache.GetInputHash("build", "/project", nil)
	assert.Equal(t, ports.HashReady, result.State)
	assert.Equal(t, "hash123", result.Hash)
}

func TestHashCache_GetInputHash_Pending(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return([]string{"/project/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	// Invalidate the cache
	cache.Invalidate([]string{"/project/main.go"})

	result := cache.GetInputHash("build", "/project", nil)
	assert.Equal(t, ports.HashPending, result.State)
	assert.Empty(t, result.Hash)
}

func TestHashCache_ComputeHash_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/**/*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"src/**/*.go"}, "/workspace").
		Return([]string{"/workspace/src/main.go", "/workspace/src/lib.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/workspace/src/main.go", "/workspace/src/lib.go"}).
		Return("computed-hash-abc", nil)

	err := cache.ComputeHash(task, "/workspace", map[string]string{"KEY": "value"})
	require.NoError(t, err)

	// Verify the hash is now available
	result := cache.GetInputHash("test-task", "/workspace", map[string]string{"KEY": "value"})
	assert.Equal(t, ports.HashReady, result.State)
	assert.Equal(t, "computed-hash-abc", result.Hash)
}

func TestHashCache_ComputeHash_ResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return(nil, errors.New("resolve failed"))

	err := cache.ComputeHash(task, "/project", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve failed")
}

func TestHashCache_ComputeHash_HasherError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("test-task"),
		Inputs: []domain.InternedString{domain.NewInternedString("*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return([]string{"/project/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/main.go"}).
		Return("", errors.New("hash computation failed"))

	err := cache.ComputeHash(task, "/project", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash computation failed")
}

func TestHashCache_ComputeHash_DifferentEnvs(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("*.go")},
	}

	// First call with env1
	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return([]string{"/project/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, map[string]string{"GO": "1.20"}, []string{"/project/main.go"}).
		Return("hash-v1.20", nil)

	err := cache.ComputeHash(task, "/project", map[string]string{"GO": "1.20"})
	require.NoError(t, err)

	// Second call with env2 - should compute different hash
	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return([]string{"/project/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, map[string]string{"GO": "1.21"}, []string{"/project/main.go"}).
		Return("hash-v1.21", nil)

	err = cache.ComputeHash(task, "/project", map[string]string{"GO": "1.21"})
	require.NoError(t, err)

	// Verify both hashes are stored separately
	result1 := cache.GetInputHash("build", "/project", map[string]string{"GO": "1.20"})
	assert.Equal(t, ports.HashReady, result1.State)
	assert.Equal(t, "hash-v1.20", result1.Hash)

	result2 := cache.GetInputHash("build", "/project", map[string]string{"GO": "1.21"})
	assert.Equal(t, ports.HashReady, result2.State)
	assert.Equal(t, "hash-v1.21", result2.Hash)
}

func TestHashCache_Invalidate_AffectedPaths(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"src/*.go"}, "/project").
		Return([]string{"/project/src/main.go", "/project/src/lib.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/src/main.go", "/project/src/lib.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	// Invalidate one of the paths
	cache.Invalidate([]string{"/project/src/main.go"})

	// Should be pending now
	result := cache.GetInputHash("build", "/project", nil)
	assert.Equal(t, ports.HashPending, result.State)
}

func TestHashCache_Invalidate_UnaffectedPaths(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"src/*.go"}, "/project").
		Return([]string{"/project/src/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/src/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	// Invalidate unrelated path
	cache.Invalidate([]string{"/project/README.md"})

	// Should still be ready
	result := cache.GetInputHash("build", "/project", nil)
	assert.Equal(t, ports.HashReady, result.State)
	assert.Equal(t, "hash123", result.Hash)
}

func TestHashCache_Invalidate_Idempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"src/*.go"}, "/project").
		Return([]string{"/project/src/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/src/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	// Invalidate multiple times
	cache.Invalidate([]string{"/project/src/main.go"})
	cache.Invalidate([]string{"/project/src/main.go"})
	cache.Invalidate([]string{"/project/src/main.go"})

	// Should only have one pending entry
	pending := cache.GetPendingTasks()
	assert.Len(t, pending, 1)
}

func TestHashCache_GetTask_Found(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:    domain.NewInternedString("test-task"),
		Command: []string{"echo", "hello"},
		Inputs:  []domain.InternedString{domain.NewInternedString("*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return([]string{"/project/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	retrieved, ok := cache.GetTask("test-task")
	require.True(t, ok)
	assert.Equal(t, "test-task", retrieved.Name.String())
	assert.Equal(t, []string{"echo", "hello"}, retrieved.Command)
}

func TestHashCache_GetTask_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	_, ok := cache.GetTask("nonexistent")
	assert.False(t, ok)
}

func TestHashCache_GetPendingTasks_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	pending := cache.GetPendingTasks()
	assert.Empty(t, pending)
}

func TestHashCache_GetPendingTasks_AfterInvalidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"src/*.go"}, "/project").
		Return([]string{"/project/src/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/src/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", map[string]string{"ENV": "prod"})
	require.NoError(t, err)

	// Invalidate
	cache.Invalidate([]string{"/project/src/main.go"})

	pending := cache.GetPendingTasks()
	require.Len(t, pending, 1)
	assert.Equal(t, "build", pending[0].TaskName)
	assert.Equal(t, "/project", pending[0].Root)
	assert.Equal(t, "prod", pending[0].Env["ENV"])
}

func TestHashCache_PendingRemovedAfterRecompute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/*.go")},
	}

	// Initial compute
	mockResolver.EXPECT().
		ResolveInputs([]string{"src/*.go"}, "/project").
		Return([]string{"/project/src/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/src/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	// Invalidate
	cache.Invalidate([]string{"/project/src/main.go"})
	assert.Len(t, cache.GetPendingTasks(), 1)

	// Recompute - should remove from pending
	mockResolver.EXPECT().
		ResolveInputs([]string{"src/*.go"}, "/project").
		Return([]string{"/project/src/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/src/main.go"}).
		Return("hash456", nil)

	err = cache.ComputeHash(task, "/project", nil)
	require.NoError(t, err)

	// Should be empty now
	assert.Empty(t, cache.GetPendingTasks())

	// And hash should be ready with new value
	result := cache.GetInputHash("build", "/project", nil)
	assert.Equal(t, ports.HashReady, result.State)
	assert.Equal(t, "hash456", result.Hash)
}

func TestHashCache_GetPendingTasks_DeepCopy(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("*.go")},
	}

	mockResolver.EXPECT().
		ResolveInputs([]string{"*.go"}, "/project").
		Return([]string{"/project/main.go"}, nil)

	mockHasher.EXPECT().
		ComputeInputHash(task, gomock.Any(), []string{"/project/main.go"}).
		Return("hash123", nil)

	err := cache.ComputeHash(task, "/project", map[string]string{"KEY": "value"})
	require.NoError(t, err)

	cache.Invalidate([]string{"/project/main.go"})

	// Get pending and modify the returned env
	pending := cache.GetPendingTasks()
	require.Len(t, pending, 1)
	pending[0].Env["KEY"] = "modified"

	// Get pending again - should not see the modification (deep copy)
	pending2 := cache.GetPendingTasks()
	assert.Equal(t, "value", pending2[0].Env["KEY"])
}

func TestHashCache_Invalidate_MultipleTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHasher := mocks.NewMockHasher(ctrl)
	mockResolver := mocks.NewMockInputResolver(ctrl)

	cache := watcher.NewHashCache(mockHasher, mockResolver)

	task1 := &domain.Task{
		Name:   domain.NewInternedString("build"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/*.go")},
	}
	task2 := &domain.Task{
		Name:   domain.NewInternedString("test"),
		Inputs: []domain.InternedString{domain.NewInternedString("src/*.go")},
	}

	// Both tasks share the same input path
	gomock.InOrder(
		mockResolver.EXPECT().
			ResolveInputs([]string{"src/*.go"}, "/project").
			Return([]string{"/project/src/main.go"}, nil),
		mockHasher.EXPECT().
			ComputeInputHash(task1, gomock.Any(), []string{"/project/src/main.go"}).
			Return("hash1", nil),
		mockResolver.EXPECT().
			ResolveInputs([]string{"src/*.go"}, "/project").
			Return([]string{"/project/src/main.go"}, nil),
		mockHasher.EXPECT().
			ComputeInputHash(task2, gomock.Any(), []string{"/project/src/main.go"}).
			Return("hash2", nil),
	)

	err := cache.ComputeHash(task1, "/project", nil)
	require.NoError(t, err)

	err = cache.ComputeHash(task2, "/project", nil)
	require.NoError(t, err)

	// Invalidate the shared path - both tasks should be pending
	cache.Invalidate([]string{"/project/src/main.go"})

	pending := cache.GetPendingTasks()
	require.Len(t, pending, 2)

	taskNames := make([]string, len(pending))
	for i, p := range pending {
		taskNames[i] = p.TaskName
	}
	assert.Contains(t, taskNames, "build")
	assert.Contains(t, taskNames, "test")
}
