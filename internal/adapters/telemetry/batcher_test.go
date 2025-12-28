package telemetry_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.trai.ch/bob/internal/adapters/telemetry"
)

func TestBatchProcessor_FlushOnSize(t *testing.T) {
	var collected []byte
	var mu sync.Mutex

	// Size limit 5 bytes.
	// We use a large time limit to ensure it doesn't trigger the flush.
	bp := telemetry.NewBatchProcessor(5, time.Hour, func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		collected = append(collected, data...)
	})
	defer func() { _ = bp.Close() }()

	// Write 3 bytes - no flush expected yet
	_, err := bp.Write([]byte("123"))
	require.NoError(t, err)

	mu.Lock()
	assert.Empty(t, collected)
	mu.Unlock()

	// Write 3 bytes - total 6 > 5. Should flush.
	// Since write calls flush synchronously when limit is reached,
	// we can assert immediately.
	_, err = bp.Write([]byte("456"))
	require.NoError(t, err)

	mu.Lock()
	assert.Equal(t, "123456", string(collected))
	mu.Unlock()
}

func TestBatchProcessor_FlushOnTime(t *testing.T) {
	var collected []byte
	var mu sync.Mutex
	flushCh := make(chan struct{}, 1)

	// Small time limit
	bp := telemetry.NewBatchProcessor(100, 50*time.Millisecond, func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		collected = append(collected, data...)
		// Signal that flush happened
		select {
		case flushCh <- struct{}{}:
		default:
		}
	})
	defer func() { _ = bp.Close() }()

	_, err := bp.Write([]byte("test"))
	require.NoError(t, err)

	mu.Lock()
	assert.Empty(t, collected)
	mu.Unlock()

	// Wait for flush
	select {
	case <-flushCh:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for flush")
	}

	mu.Lock()
	assert.Equal(t, "test", string(collected))
	mu.Unlock()
}

func TestBatchProcessor_ManualFlush(t *testing.T) {
	var collected []byte
	var mu sync.Mutex

	bp := telemetry.NewBatchProcessor(100, time.Hour, func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		collected = append(collected, data...)
	})
	defer func() { _ = bp.Close() }()

	_, err := bp.Write([]byte("hello"))
	require.NoError(t, err)

	mu.Lock()
	assert.Empty(t, collected)
	mu.Unlock()

	bp.Flush()

	mu.Lock()
	assert.Equal(t, "hello", string(collected))
	mu.Unlock()
}

func TestBatchProcessor_CloseFlushes(t *testing.T) {
	var collected []byte
	var mu sync.Mutex

	bp := telemetry.NewBatchProcessor(100, time.Hour, func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		collected = append(collected, data...)
	})

	_, err := bp.Write([]byte("pending"))
	require.NoError(t, err)

	mu.Lock()
	assert.Empty(t, collected)
	mu.Unlock()

	err = bp.Close()
	require.NoError(t, err)

	mu.Lock()
	assert.Equal(t, "pending", string(collected))
	mu.Unlock()

	// Write after close should fail
	_, err = bp.Write([]byte("fail"))
	assert.Error(t, err)
}

func TestBatchProcessor_ThreadSafety(t *testing.T) {
	var collected []byte
	var mu sync.Mutex

	// Use small limits to trigger frequent flushing from both size and time
	bp := telemetry.NewBatchProcessor(20, 10*time.Millisecond, func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		collected = append(collected, data...)
	})
	defer func() { _ = bp.Close() }()

	var wg sync.WaitGroup
	workers := 10
	iterations := 100
	data := []byte("a")

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = bp.Write(data)
				if j%10 == 0 {
					// occasional manual flush
					bp.Flush()
				}
				// occasional sleep to allow time-based flush
				if j%20 == 0 {
					time.Sleep(1 * time.Millisecond) // Short sleep should rely on system timer
				}
			}
		}()
	}

	wg.Wait()
	_ = bp.Close()

	mu.Lock()
	// Logic check: We wrote 1 byte 'a' * workers * iterations times.
	// Total bytes should match exactly.
	assert.Len(t, collected, workers*iterations)
	mu.Unlock()
}
