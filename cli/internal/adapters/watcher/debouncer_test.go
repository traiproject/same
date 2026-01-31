package watcher_test

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/internal/adapters/watcher"
)

func TestNewDebouncer(t *testing.T) {
	tests := []struct {
		name     string
		window   time.Duration
		callback func([]string)
	}{
		{
			name:     "with callback",
			window:   100 * time.Millisecond,
			callback: func([]string) {},
		},
		{
			name:     "with nil callback",
			window:   50 * time.Millisecond,
			callback: nil,
		},
		{
			name:     "with zero window",
			window:   0,
			callback: func([]string) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := watcher.NewDebouncer(tt.window, tt.callback)
			require.NotNil(t, d)
		})
	}
}

func TestDebouncer_Add_SinglePath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int
		var receivedPaths []string

		d := watcher.NewDebouncer(100*time.Millisecond, func(paths []string) {
			callCount++
			receivedPaths = paths
		})

		d.Add("/project/src/main.go")

		// Advance time past the debounce window
		time.Sleep(150 * time.Millisecond)
		synctest.Wait()

		require.Equal(t, 1, callCount)
		require.Len(t, receivedPaths, 1)
		assert.Equal(t, "/project/src/main.go", receivedPaths[0])
	})
}

func TestDebouncer_Add_MultiplePathsCoalesced(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int
		var receivedPaths []string

		d := watcher.NewDebouncer(100*time.Millisecond, func(paths []string) {
			callCount++
			receivedPaths = paths
		})

		// Add multiple paths within the debounce window
		d.Add("/project/src/file1.go")
		d.Add("/project/src/file2.go")
		d.Add("/project/src/file3.go")

		// Advance time past the debounce window
		time.Sleep(150 * time.Millisecond)
		synctest.Wait()

		// Should only be called once with all paths
		require.Equal(t, 1, callCount)
		require.Len(t, receivedPaths, 3)

		// Paths should be deduplicated (interned handles).
		// Order is not guaranteed since paths are stored in a map.
		assert.Contains(t, receivedPaths, "/project/src/file1.go")
		assert.Contains(t, receivedPaths, "/project/src/file2.go")
		assert.Contains(t, receivedPaths, "/project/src/file3.go")
	})
}

func TestDebouncer_Add_DuplicatePaths(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int
		var receivedPaths []string

		d := watcher.NewDebouncer(100*time.Millisecond, func(paths []string) {
			callCount++
			receivedPaths = paths
		})

		// Add the same path multiple times
		d.Add("/project/src/main.go")
		d.Add("/project/src/main.go")
		d.Add("/project/src/main.go")

		// Advance time past the debounce window
		time.Sleep(150 * time.Millisecond)
		synctest.Wait()

		require.Equal(t, 1, callCount)
		// Duplicate paths should be deduplicated via unique.Handle
		require.Len(t, receivedPaths, 1)
		assert.Equal(t, "/project/src/main.go", receivedPaths[0])
	})
}

func TestDebouncer_Add_TimerReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int
		var mu sync.Mutex

		d := watcher.NewDebouncer(100*time.Millisecond, func([]string) {
			mu.Lock()
			callCount++
			mu.Unlock()
		})

		// First add starts the timer
		d.Add("/project/src/file1.go")
		time.Sleep(50 * time.Millisecond)

		// Second add resets the timer
		d.Add("/project/src/file2.go")
		time.Sleep(50 * time.Millisecond)

		// At this point (100ms from first add), if timer wasn't reset,
		// the callback would have fired. But it should not have fired yet.
		synctest.Wait()
		mu.Lock()
		count := callCount
		mu.Unlock()
		assert.Equal(t, 0, count)

		// Wait for the reset timer to fire
		time.Sleep(60 * time.Millisecond)
		synctest.Wait()

		mu.Lock()
		count = callCount
		mu.Unlock()
		require.Equal(t, 1, count)
	})
}

func TestDebouncer_Flush_Immediate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int
		var receivedPaths []string

		d := watcher.NewDebouncer(100*time.Millisecond, func(paths []string) {
			callCount++
			receivedPaths = paths
		})

		d.Add("/project/src/file1.go")
		d.Add("/project/src/file2.go")

		// Flush immediately, before timer fires
		d.Flush()

		// Callback should have been called synchronously
		require.Equal(t, 1, callCount)
		require.Len(t, receivedPaths, 2)
		assert.Contains(t, receivedPaths, "/project/src/file1.go")
		assert.Contains(t, receivedPaths, "/project/src/file2.go")
	})
}

func TestDebouncer_Flush_Empty(t *testing.T) {
	var callCount int

	d := watcher.NewDebouncer(100*time.Millisecond, func([]string) {
		callCount++
	})

	// Flush without any pending paths
	d.Flush()

	// Callback should not have been called
	assert.Equal(t, 0, callCount)
}

func TestDebouncer_Flush_AfterFire(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int

		d := watcher.NewDebouncer(50*time.Millisecond, func([]string) {
			callCount++
		})

		d.Add("/project/src/file1.go")

		// Wait for timer to fire
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		require.Equal(t, 1, callCount)

		// Flush after timer already fired - should not call again
		d.Flush()

		assert.Equal(t, 1, callCount)
	})
}

func TestDebouncer_NilCallback(t *testing.T) {
	synctest.Test(t, func(_ *testing.T) {
		d := watcher.NewDebouncer(50*time.Millisecond, nil)

		// Should not panic when adding paths
		d.Add("/project/src/file1.go")
		d.Add("/project/src/file2.go")

		// Wait for timer
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		// Flush should also not panic
		d.Flush()
	})
}

func TestDebouncer_Add_AfterFlush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int
		var receivedPaths []string

		d := watcher.NewDebouncer(100*time.Millisecond, func(paths []string) {
			callCount++
			receivedPaths = paths
		})

		// First batch
		d.Add("/project/src/file1.go")
		d.Flush()

		require.Equal(t, 1, callCount)
		require.Len(t, receivedPaths, 1)

		// Second batch after flush
		d.Add("/project/src/file2.go")
		d.Add("/project/src/file3.go")

		time.Sleep(150 * time.Millisecond)
		synctest.Wait()

		require.Equal(t, 2, callCount)
		require.Len(t, receivedPaths, 2)
		assert.Contains(t, receivedPaths, "/project/src/file2.go")
		assert.Contains(t, receivedPaths, "/project/src/file3.go")
	})
}

func TestDebouncer_Flush_ClearsPending(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callCount int

		d := watcher.NewDebouncer(100*time.Millisecond, func([]string) {
			callCount++
		})

		d.Add("/project/src/file1.go")
		d.Flush()

		require.Equal(t, 1, callCount)

		// Wait for original timer - should not trigger another call
		time.Sleep(150 * time.Millisecond)
		synctest.Wait()

		assert.Equal(t, 1, callCount)
	})
}
