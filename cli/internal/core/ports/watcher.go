package ports

import (
	"context"
	"iter"
)

// WatchOp represents the type of file system operation.
type WatchOp uint8

const (
	// OpCreate indicates a file or directory was created.
	OpCreate WatchOp = iota
	// OpWrite indicates a file was modified.
	OpWrite
	// OpRemove indicates a file or directory was removed.
	OpRemove
	// OpRename indicates a file or directory was renamed.
	OpRename
)

// WatchEvent represents a file system event from the watcher.
type WatchEvent struct {
	// Path is the absolute path of the file or directory that changed.
	Path string
	// Operation is the type of change that occurred.
	Operation WatchOp
}

// Watcher defines the interface for watching file system changes.
type Watcher interface {
	// Start begins watching the given root directory recursively.
	// It returns an error if the watcher fails to start.
	Start(ctx context.Context, root string) error
	// Stop stops the watcher and releases all resources.
	Stop() error
	// Events returns an iterator of file system events.
	Events() iter.Seq[WatchEvent]
}
