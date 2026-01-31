// Package watcher implements file system watching for proactive input hashing.
package watcher

import (
	"sync"
	"time"
	"unique"
)

// Debouncer coalesces rapid file system events into batched invalidations.
type Debouncer struct {
	mu       sync.Mutex
	pending  map[unique.Handle[string]]struct{}
	timer    *time.Timer
	window   time.Duration
	callback func(paths []string)
}

// NewDebouncer creates a new debouncer with the given time window and callback.
func NewDebouncer(window time.Duration, callback func(paths []string)) *Debouncer {
	return &Debouncer{
		pending:  make(map[unique.Handle[string]]struct{}),
		window:   window,
		callback: callback,
	}
}

// Add adds a file path to the pending events set.
func (d *Debouncer) Add(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Add the path to the pending set using an interned handle for deduplication.
	handle := unique.Make(path)
	d.pending[handle] = struct{}{}

	// Reset the timer if it exists, or create a new one.
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.window, d.fire)
}

// fire is called when the debounce window expires.
func (d *Debouncer) fire() {
	d.mu.Lock()

	// Check if there's anything to process (protects against race with Flush).
	if len(d.pending) == 0 {
		d.timer = nil
		d.mu.Unlock()
		return
	}

	// Convert the pending set to a slice of paths.
	paths := make([]string, 0, len(d.pending))
	for handle := range d.pending {
		paths = append(paths, handle.Value())
	}

	// Clear the pending set and timer.
	d.pending = make(map[unique.Handle[string]]struct{})
	d.timer = nil
	d.mu.Unlock()

	// Call the callback with the coalesced paths (asynchronously to match Flush behavior).
	if len(paths) > 0 && d.callback != nil {
		go d.callback(paths)
	}
}

// Flush immediately triggers the debounce callback with all pending paths.
// This method blocks until the callback completes, making it suitable for
// graceful shutdown scenarios where work must finish before proceeding.
func (d *Debouncer) Flush() {
	d.mu.Lock()
	if d.timer != nil {
		if !d.timer.Stop() {
			// Timer already fired, let it complete rather than processing twice.
			d.mu.Unlock()
			return
		}
		d.timer = nil
	}

	// Extract paths to process.
	paths := make([]string, 0, len(d.pending))
	for handle := range d.pending {
		paths = append(paths, handle.Value())
	}
	d.pending = make(map[unique.Handle[string]]struct{})
	d.mu.Unlock()

	// Call the callback synchronously (blocks until complete).
	// This differs from fire() which is async, but is intentional for
	// flush scenarios where completion is required before proceeding.
	if len(paths) > 0 && d.callback != nil {
		d.callback(paths)
	}
}
