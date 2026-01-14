// Package telemetry provides adapters for collecting and processing telemetry data.
package telemetry

import (
	"bytes"
	"errors"
	"sync"
	"time"
)

const (
	// DefaultSizeLimit is the default buffer size (4KB) if not specified.
	DefaultSizeLimit = 4096
	// DefaultTimeLimit is the default flush interval (50ms) if not specified.
	DefaultTimeLimit = 50 * time.Millisecond
)

// BatchProcessor buffers writes until a size limit or time limit is reached.
// It is thread-safe.
type BatchProcessor struct {
	// configuration
	sizeLimit int
	timeLimit time.Duration
	onFlush   func([]byte)

	// synchronization
	mu     sync.Mutex
	buffer *bytes.Buffer
	ticker *time.Ticker
	stopCh chan struct{}
	closed bool
}

// NewBatchProcessor returns a new BatchProcessor.
// sizeLimit: max bytes before automatic flush.
// timeLimit: max time before automatic flush.
// onFlush: callback triggered when data is flushed.
// Call Close() to stop the background ticker.
func NewBatchProcessor(sizeLimit int, timeLimit time.Duration, onFlush func([]byte)) *BatchProcessor {
	if sizeLimit <= 0 {
		sizeLimit = DefaultSizeLimit
	}
	if timeLimit <= 0 {
		timeLimit = DefaultTimeLimit
	}

	bp := &BatchProcessor{
		sizeLimit: sizeLimit,
		timeLimit: timeLimit,
		onFlush:   onFlush,
		buffer:    new(bytes.Buffer),
		stopCh:    make(chan struct{}),
	}

	bp.ticker = time.NewTicker(timeLimit)
	go bp.run()

	return bp
}

// Write writes data to the buffer.
// If the buffer exceeds sizeLimit, it triggers a Flush.
func (bp *BatchProcessor) Write(p []byte) (n int, err error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.closed {
		return 0, errors.New("BatchProcessor is closed")
	}

	n, err = bp.buffer.Write(p)
	if err != nil {
		return n, err
	}

	if bp.buffer.Len() >= bp.sizeLimit {
		bp.flushLocked()
		// Reset ticker so we don't flush again immediately if we just flushed a full buffer
		bp.ticker.Reset(bp.timeLimit)
	}

	return n, nil
}

// Flush forces any buffered data to be sent to the callback.
func (bp *BatchProcessor) Flush() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	if bp.closed {
		return
	}
	bp.flushLocked()
}

// Close stops the background flusher and performs a final flush.
func (bp *BatchProcessor) Close() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.closed {
		return nil
	}

	bp.closed = true
	close(bp.stopCh)
	bp.flushLocked()
	return nil
}

func (bp *BatchProcessor) run() {
	for {
		select {
		case <-bp.ticker.C:
			bp.Flush()
		case <-bp.stopCh:
			bp.ticker.Stop()
			return
		}
	}
}

// flushLocked must be called with mu held.
func (bp *BatchProcessor) flushLocked() {
	if bp.buffer.Len() == 0 {
		return
	}

	// Create a copy of the data to pass to the callback
	// so that we can reset the buffer immediately.
	data := make([]byte, bp.buffer.Len())
	copy(data, bp.buffer.Bytes())
	bp.buffer.Reset()

	// Using the callback while holding the lock ensures order,
	// but creates a risk of blocking if onFlush is slow.
	// We assume onFlush is fast (e.g., sending to a channel).
	if bp.onFlush != nil {
		bp.onFlush(data)
	}
}
