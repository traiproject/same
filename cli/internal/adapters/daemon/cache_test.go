package daemon_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/daemon"
	"go.trai.ch/same/internal/core/domain"
)

func TestNewServerCache(t *testing.T) {
	cache := daemon.NewServerCache()
	require.NotNil(t, cache)
}

func TestServerCache_GetGraph_Miss(t *testing.T) {
	cache := daemon.NewServerCache()

	graph, ok := cache.GetGraph("/unknown/cwd", map[string]int64{})

	assert.False(t, ok)
	assert.Nil(t, graph)
}

func TestServerCache_GetGraph_Hit(t *testing.T) {
	cache := daemon.NewServerCache()

	graph := domain.NewGraph()
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 1234567890},
	}

	cache.SetGraph("/project", entry)

	retrieved, ok := cache.GetGraph("/project", map[string]int64{"/project/same.yaml": 1234567890})

	assert.True(t, ok)
	assert.NotNil(t, retrieved)
}

func TestServerCache_GetGraph_MtimeMismatch(t *testing.T) {
	cache := daemon.NewServerCache()

	graph := domain.NewGraph()
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 1234567890},
	}

	cache.SetGraph("/project", entry)

	// Different mtime should cause cache miss
	retrieved, ok := cache.GetGraph("/project", map[string]int64{"/project/same.yaml": 9999999999})

	assert.False(t, ok)
	assert.Nil(t, retrieved)
}

func TestServerCache_GetGraph_MtimeLengthMismatch(t *testing.T) {
	cache := daemon.NewServerCache()

	graph := domain.NewGraph()
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: []string{"/project/same.yaml", "/project/same.work.yaml"},
		Mtimes: map[string]int64{
			"/project/same.yaml":      1234567890,
			"/project/same.work.yaml": 1234567891,
		},
	}

	cache.SetGraph("/project", entry)

	// Different number of mtimes should cause cache miss
	retrieved, ok := cache.GetGraph("/project", map[string]int64{"/project/same.yaml": 1234567890})

	assert.False(t, ok)
	assert.Nil(t, retrieved)
}

func TestServerCache_GetGraph_MtimeExtraPath(t *testing.T) {
	cache := daemon.NewServerCache()

	graph := domain.NewGraph()
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 1234567890},
	}

	cache.SetGraph("/project", entry)

	// Client has an extra path not in cache should cause miss
	retrieved, ok := cache.GetGraph("/project", map[string]int64{
		"/project/same.yaml":  1234567890,
		"/project/extra.yaml": 1234567891,
	})

	assert.False(t, ok)
	assert.Nil(t, retrieved)
}

func TestServerCache_SetGraph_Overwrite(t *testing.T) {
	cache := daemon.NewServerCache()

	graph1 := domain.NewGraph()
	entry1 := &domain.GraphCacheEntry{
		Graph:       graph1,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 1111111111},
	}

	graph2 := domain.NewGraph()
	entry2 := &domain.GraphCacheEntry{
		Graph:       graph2,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 2222222222},
	}

	// Set initial entry
	cache.SetGraph("/project", entry1)

	// Overwrite with new entry
	cache.SetGraph("/project", entry2)

	// Should get the new entry
	retrieved, ok := cache.GetGraph("/project", map[string]int64{"/project/same.yaml": 2222222222})
	assert.True(t, ok)
	assert.NotNil(t, retrieved)

	// Old entry should not be retrievable
	_, ok = cache.GetGraph("/project", map[string]int64{"/project/same.yaml": 1111111111})
	assert.False(t, ok)
}

func TestServerCache_GetEnv_Miss(t *testing.T) {
	cache := daemon.NewServerCache()

	env, ok := cache.GetEnv("unknown-env-id")

	assert.False(t, ok)
	assert.Nil(t, env)
}

func TestServerCache_GetEnv_Hit(t *testing.T) {
	cache := daemon.NewServerCache()

	envVars := []string{"PATH=/usr/bin", "HOME=/home/user"}
	cache.SetEnv("env-123", envVars)

	retrieved, ok := cache.GetEnv("env-123")

	assert.True(t, ok)
	assert.Equal(t, envVars, retrieved)
}

func TestServerCache_SetEnv_Overwrite(t *testing.T) {
	cache := daemon.NewServerCache()

	env1 := []string{"PATH=/usr/bin"}
	env2 := []string{"PATH=/usr/local/bin"}

	cache.SetEnv("env-123", env1)
	cache.SetEnv("env-123", env2)

	retrieved, ok := cache.GetEnv("env-123")
	assert.True(t, ok)
	assert.Equal(t, env2, retrieved)
}

func TestServerCache_Concurrent_GraphAccess(t *testing.T) {
	cache := daemon.NewServerCache()

	graph := domain.NewGraph()
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 1234567890},
	}

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			cwd := fmt.Sprintf("/cwd/%d", idx)
			cache.SetGraph(cwd, entry)
		}(i)

		go func(idx int) {
			defer wg.Done()
			cwd := fmt.Sprintf("/cwd/%d", idx)
			retrieved, ok := cache.GetGraph(cwd, map[string]int64{"/project/same.yaml": 1234567890})
			if ok {
				assert.NotNil(t, retrieved)
			}
		}(i)
	}

	wg.Wait()

	// Verify all entries were written correctly
	for i := range goroutines {
		cwd := fmt.Sprintf("/cwd/%d", i)
		retrieved, ok := cache.GetGraph(cwd, map[string]int64{"/project/same.yaml": 1234567890})
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
	}
}

func TestServerCache_Concurrent_EnvAccess(t *testing.T) {
	cache := daemon.NewServerCache()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	envVars := []string{"KEY=value"}

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			envID := fmt.Sprintf("env-%d", idx)
			cache.SetEnv(envID, envVars)
		}(i)

		go func(idx int) {
			defer wg.Done()
			envID := fmt.Sprintf("env-%d", idx)
			retrieved, ok := cache.GetEnv(envID)
			if ok {
				assert.Equal(t, envVars, retrieved)
			}
		}(i)
	}

	wg.Wait()

	// Verify all entries were written correctly
	for i := range goroutines {
		envID := fmt.Sprintf("env-%d", i)
		retrieved, ok := cache.GetEnv(envID)
		assert.True(t, ok)
		assert.Equal(t, envVars, retrieved)
	}
}

func TestServerCache_Concurrent_MixedAccess(t *testing.T) {
	cache := daemon.NewServerCache()

	graph := domain.NewGraph()
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 1234567890},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for i := range goroutines {
		// Graph writers
		go func(idx int) {
			defer wg.Done()
			cwd := fmt.Sprintf("/cwd/%d", idx)
			cache.SetGraph(cwd, entry)
		}(i)

		// Graph readers
		go func(idx int) {
			defer wg.Done()
			cwd := fmt.Sprintf("/cwd/%d", idx)
			cache.GetGraph(cwd, map[string]int64{"/project/same.yaml": 1234567890})
		}(i)

		// Env writers
		go func(idx int) {
			defer wg.Done()
			envID := fmt.Sprintf("env-%d", idx)
			cache.SetEnv(envID, []string{"KEY=value"})
		}(i)

		// Env readers
		go func(idx int) {
			defer wg.Done()
			envID := fmt.Sprintf("env-%d", idx)
			cache.GetEnv(envID)
		}(i)
	}

	wg.Wait()

	// Verify correctness of both graph and env caches
	for i := range goroutines {
		cwd := fmt.Sprintf("/cwd/%d", i)
		retrieved, ok := cache.GetGraph(cwd, map[string]int64{"/project/same.yaml": 1234567890})
		assert.True(t, ok)
		assert.NotNil(t, retrieved)

		envID := fmt.Sprintf("env-%d", i)
		env, envOk := cache.GetEnv(envID)
		assert.True(t, envOk)
		assert.Equal(t, []string{"KEY=value"}, env)
	}
}

func TestServerCache_MultipleCwdIsolation(t *testing.T) {
	cache := daemon.NewServerCache()

	graph1 := domain.NewGraph()
	entry1 := &domain.GraphCacheEntry{
		Graph:       graph1,
		ConfigPaths: []string{"/project1/same.yaml"},
		Mtimes:      map[string]int64{"/project1/same.yaml": 1111111111},
	}

	graph2 := domain.NewGraph()
	entry2 := &domain.GraphCacheEntry{
		Graph:       graph2,
		ConfigPaths: []string{"/project2/same.yaml"},
		Mtimes:      map[string]int64{"/project2/same.yaml": 2222222222},
	}

	cache.SetGraph("/project1", entry1)
	cache.SetGraph("/project2", entry2)

	// Verify each project can retrieve its own graph
	retrieved1, ok1 := cache.GetGraph("/project1", map[string]int64{"/project1/same.yaml": 1111111111})
	assert.True(t, ok1)
	assert.NotNil(t, retrieved1)

	retrieved2, ok2 := cache.GetGraph("/project2", map[string]int64{"/project2/same.yaml": 2222222222})
	assert.True(t, ok2)
	assert.NotNil(t, retrieved2)

	// Verify cross-project access fails
	_, cross1 := cache.GetGraph("/project1", map[string]int64{"/project2/same.yaml": 2222222222})
	assert.False(t, cross1)

	_, cross2 := cache.GetGraph("/project2", map[string]int64{"/project1/same.yaml": 1111111111})
	assert.False(t, cross2)
}

func TestServerCache_MultipleEnvIsolation(t *testing.T) {
	cache := daemon.NewServerCache()

	env1 := []string{"PROJECT=1"}
	env2 := []string{"PROJECT=2"}

	cache.SetEnv("env-project1", env1)
	cache.SetEnv("env-project2", env2)

	// Verify each envID retrieves its own env
	retrieved1, ok1 := cache.GetEnv("env-project1")
	assert.True(t, ok1)
	assert.Equal(t, env1, retrieved1)

	retrieved2, ok2 := cache.GetEnv("env-project2")
	assert.True(t, ok2)
	assert.Equal(t, env2, retrieved2)
}

func TestServerCache_EmptyMtimes(t *testing.T) {
	cache := daemon.NewServerCache()

	graph := domain.NewGraph()
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: []string{},
		Mtimes:      map[string]int64{},
	}

	cache.SetGraph("/project", entry)

	// Empty mtimes should match
	retrieved, ok := cache.GetGraph("/project", map[string]int64{})
	assert.True(t, ok)
	assert.NotNil(t, retrieved)
}

func TestServerCache_NilGraph(t *testing.T) {
	cache := daemon.NewServerCache()

	entry := &domain.GraphCacheEntry{
		Graph:       nil,
		ConfigPaths: []string{"/project/same.yaml"},
		Mtimes:      map[string]int64{"/project/same.yaml": 1234567890},
	}

	cache.SetGraph("/project", entry)

	retrieved, ok := cache.GetGraph("/project", map[string]int64{"/project/same.yaml": 1234567890})
	assert.True(t, ok)
	assert.Nil(t, retrieved)
}

func TestServerCache_NilEnvSlice(t *testing.T) {
	cache := daemon.NewServerCache()

	// Setting nil env slice
	cache.SetEnv("env-nil", nil)

	retrieved, ok := cache.GetEnv("env-nil")
	assert.True(t, ok)
	assert.Nil(t, retrieved)
}

func TestServerCache_Concurrent_SameKeyGraphContention(t *testing.T) {
	cache := daemon.NewServerCache()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// All goroutines contend on the same key
	const sharedCwd = "/shared/project"

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			graph := domain.NewGraph()
			entry := &domain.GraphCacheEntry{
				Graph:       graph,
				ConfigPaths: []string{"/project/same.yaml"},
				Mtimes:      map[string]int64{"/project/same.yaml": int64(1000000 + idx)},
			}
			cache.SetGraph(sharedCwd, entry)
		}(i)

		go func(_ int) {
			defer wg.Done()
			cache.GetGraph(sharedCwd, map[string]int64{"/project/same.yaml": 1000000})
		}(i)
	}

	wg.Wait()

	// Verify the cache has one entry for the shared key
	// We can't predict which goroutine won the race, so we verify that
	// at least one of the possible mtimes returns a valid entry
	foundValid := false
	for i := range goroutines {
		mtime := int64(1000000 + i)
		if retrieved, ok := cache.GetGraph(sharedCwd, map[string]int64{"/project/same.yaml": mtime}); ok {
			assert.NotNil(t, retrieved)
			foundValid = true
			break
		}
	}
	assert.True(t, foundValid, "cache should contain a valid entry for shared key")
}

func TestServerCache_Concurrent_SameKeyEnvContention(t *testing.T) {
	cache := daemon.NewServerCache()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// All goroutines contend on the same key
	const sharedEnvID = "shared-env"

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			envVars := []string{fmt.Sprintf("INDEX=%d", idx)}
			cache.SetEnv(sharedEnvID, envVars)
		}(i)

		go func(_ int) {
			defer wg.Done()
			cache.GetEnv(sharedEnvID)
		}(i)
	}

	wg.Wait()

	// Verify the cache has one entry for the shared key
	retrieved, ok := cache.GetEnv(sharedEnvID)
	assert.True(t, ok)
	// The exact value depends on which goroutine won the race, but should be valid
	assert.NotNil(t, retrieved)
	assert.Len(t, retrieved, 1)
	assert.Contains(t, retrieved[0], "INDEX=")
}
