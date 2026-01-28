package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"unique"

	"github.com/fsnotify/fsnotify"
	"go.trai.ch/same/internal/core/ports"
)

var _ ports.Watcher = (*Watcher)(nil)

// shouldSkipDirectories are directories that should not be watched.
var shouldSkipDirectories = map[string]bool{
	".git":         true,
	".jj":          true,
	"node_modules": true,
}

const eventChannelBuffer = 100

// Watcher implements file system watching using fsnotify.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	root      unique.Handle[string]
	events    chan ports.WatchEvent
}

// NewWatcher creates a new file system watcher.
func NewWatcher() (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fsWatcher: watcher,
		events:    make(chan ports.WatchEvent, eventChannelBuffer),
	}, nil
}

// Start begins watching the given root directory recursively.
func (w *Watcher) Start(ctx context.Context, root string) error {
	w.root = unique.Make(root)

	// Walk the directory tree and add all directories to the watcher.
	for dir := range w.watchRecursively(root) {
		if err := w.fsWatcher.Add(dir); err != nil {
			return err
		}
	}

	// Start processing events in a goroutine.
	go w.processEvents(ctx)

	return nil
}

// Stop stops the watcher and releases all resources.
func (w *Watcher) Stop() error {
	return w.fsWatcher.Close()
}

// Events returns an iterator of file system events.
func (w *Watcher) Events() iter.Seq[ports.WatchEvent] {
	return func(yield func(ports.WatchEvent) bool) {
		for event := range w.events {
			if !yield(event) {
				return
			}
		}
	}
}

// watchRecursively walks the directory tree and yields all directories.
func (w *Watcher) watchRecursively(root string) iter.Seq[string] {
	return func(yield func(string) bool) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// Continue walking even if there's an error accessing a directory.
				return nil //nolint:nilerr // This is intentional - we want to skip problematic directories
			}
			if d.IsDir() {
				if w.shouldSkip(d.Name()) {
					return fs.SkipDir
				}
				if !yield(path) {
					return filepath.SkipAll
				}
			}
			return nil
		})
	}
}

// shouldSkip returns true if the directory should be skipped.
func (w *Watcher) shouldSkip(name string) bool {
	return shouldSkipDirectories[name]
}

// processEvents processes raw fsnotify events and converts them to ports.WatchEvent.
//
//nolint:cyclop // This function is complex due to multiple event types and error handling
func (w *Watcher) processEvents(ctx context.Context) {
	defer close(w.events)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Convert fsnotify event to ports.WatchEvent.
			watchEvent := w.convertEvent(event)
			if watchEvent == nil {
				continue
			}

			// Send the event to the output channel.
			select {
			case w.events <- *watchEvent:
			case <-ctx.Done():
				return
			}

			// If a new directory was created, add it to the watcher.
			if event.Op&fsnotify.Create == fsnotify.Create && watchEvent.Operation == ports.OpCreate {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() && !w.shouldSkip(info.Name()) {
					// Recursively add the new directory and its subdirectories.
					for dir := range w.watchRecursively(event.Name) {
						_ = w.fsWatcher.Add(dir)
					}
				}
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// Log error to stderr and continue processing.
			fmt.Fprintf(os.Stderr, "watcher: file system error: %v\n", err)
		}
	}
}

// convertEvent converts an fsnotify event to a ports.WatchEvent.
func (w *Watcher) convertEvent(event fsnotify.Event) *ports.WatchEvent {
	path := event.Name

	if event.Op&fsnotify.Write == fsnotify.Write {
		return &ports.WatchEvent{
			Path:      path,
			Operation: ports.OpWrite,
		}
	}

	if event.Op&fsnotify.Create == fsnotify.Create {
		return &ports.WatchEvent{
			Path:      path,
			Operation: ports.OpCreate,
		}
	}

	if event.Op&fsnotify.Remove == fsnotify.Remove {
		return &ports.WatchEvent{
			Path:      path,
			Operation: ports.OpRemove,
		}
	}

	if event.Op&fsnotify.Rename == fsnotify.Rename {
		return &ports.WatchEvent{
			Path:      path,
			Operation: ports.OpRename,
		}
	}

	return nil
}
