package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
	"unique"

	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
)

// PendingRehash represents a cache entry that needs to be recomputed.
type PendingRehash struct {
	TaskName string
	Root     string
	Env      map[string]string
}

// HashCache implements ports.InputHashCache with background rehashing.
type HashCache struct {
	mu              sync.RWMutex
	entries         map[unique.Handle[string]]*domain.TaskHashEntry
	pathToTasks     map[unique.Handle[string]][]cacheEntry
	cacheKeyContext map[unique.Handle[string]]PendingRehash // Maps cache key to its context for invalidation
	pendingRehashes []PendingRehash                         // Track what needs rehashing with full context
	pendingKeys     map[unique.Handle[string]]struct{}      // O(1) pending lookup set
	tasks           map[unique.Handle[string]]*domain.Task  // Full task definitions
	hasher          ports.Hasher
	resolver        ports.InputResolver
}

// cacheEntry links a path to a cache key for invalidation.
type cacheEntry struct {
	cacheKey unique.Handle[string]
}

// NewHashCache creates a new hash cache.
func NewHashCache(hasher ports.Hasher, resolver ports.InputResolver) *HashCache {
	return &HashCache{
		entries:         make(map[unique.Handle[string]]*domain.TaskHashEntry),
		pathToTasks:     make(map[unique.Handle[string]][]cacheEntry),
		cacheKeyContext: make(map[unique.Handle[string]]PendingRehash),
		pendingRehashes: make([]PendingRehash, 0),
		pendingKeys:     make(map[unique.Handle[string]]struct{}),
		tasks:           make(map[unique.Handle[string]]*domain.Task),
		hasher:          hasher,
		resolver:        resolver,
	}
}

// copyEnv creates a deep copy of an environment map to prevent shared reference bugs.
func copyEnv(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}
	copied := make(map[string]string, len(env))
	for k, v := range env {
		copied[k] = v
	}
	return copied
}

// makeCacheKey creates a unique cache key from task name, root, and environment.
// This ensures different contexts don't collide in the cache.
// Uses a truncated SHA-256 hash (64 bits) of the environment for space efficiency.
// Collision probability is negligible for typical daemon workloads.
func (h *HashCache) makeCacheKey(taskName, root string, env map[string]string) unique.Handle[string] {
	// Sort environment keys for deterministic hashing
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build environment string
	envStr := ""
	for _, k := range keys {
		envStr += fmt.Sprintf("%s=%s;", k, env[k])
	}

	// Hash the environment to keep key size reasonable
	envHash := sha256.Sum256([]byte(envStr))
	envHashStr := hex.EncodeToString(envHash[:8]) // Use first 8 bytes (64 bits)

	// Combine task name, root, and env hash
	cacheKey := fmt.Sprintf("%s|%s|%s", taskName, root, envHashStr)
	return unique.Make(cacheKey)
}

// GetInputHash returns the current hash state and value for the given task.
func (h *HashCache) GetInputHash(taskName, root string, env map[string]string) ports.InputHashResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cacheKey := h.makeCacheKey(taskName, root, env)

	// Check if this specific context is pending rehash using O(1) set lookup.
	if _, pending := h.pendingKeys[cacheKey]; pending {
		return ports.InputHashResult{State: ports.HashPending}
	}

	// Check if we have a cached entry.
	if entry, ok := h.entries[cacheKey]; ok {
		return ports.InputHashResult{
			State: ports.HashReady,
			Hash:  entry.Hash,
		}
	}

	// Task is not cached yet.
	return ports.InputHashResult{State: ports.HashUnknown}
}

// Invalidate marks cached hashes for tasks affected by the changed paths.
// For each affected cache entry, we delete it and add it to the pending list for background rehashing.
func (h *HashCache) Invalidate(paths []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// For each changed path, find all cache entries that depend on it.
	for _, path := range paths {
		pathHandle := unique.Make(path)
		if entries, ok := h.pathToTasks[pathHandle]; ok {
			for _, entry := range entries {
				// Look up the full context for this cache key
				if context, ok := h.cacheKeyContext[entry.cacheKey]; ok {
					// Add to pending rehashes if not already there (O(1) check with pendingKeys)
					if _, exists := h.pendingKeys[entry.cacheKey]; !exists {
						// Deep copy the env map to prevent shared reference bugs
						h.pendingRehashes = append(h.pendingRehashes, PendingRehash{
							TaskName: context.TaskName,
							Root:     context.Root,
							Env:      copyEnv(context.Env),
						})
						h.pendingKeys[entry.cacheKey] = struct{}{}
					}
				}

				// Delete the stale cache entry
				delete(h.entries, entry.cacheKey)
			}
		}
	}
}

// GetTask retrieves the stored task definition by name.
// This is used by background workers to rehash pending tasks.
func (h *HashCache) GetTask(taskName string) (*domain.Task, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	handle := unique.Make(taskName)
	task, ok := h.tasks[handle]
	return task, ok
}

// ComputeHash computes and caches the hash for a task with the given context.
// This should be called to populate the cache for a specific task/root/env combination.
func (h *HashCache) ComputeHash(task *domain.Task, root string, env map[string]string) error {
	// Extract string inputs from task.
	inputs := make([]string, len(task.Inputs))
	for i, input := range task.Inputs {
		inputs[i] = input.String()
	}

	// Resolve inputs to concrete paths.
	resolved, err := h.resolver.ResolveInputs(inputs, root)
	if err != nil {
		return err
	}

	// Compute the hash with the full task.
	hash, err := h.hasher.ComputeInputHash(task, env, resolved)
	if err != nil {
		return err
	}

	// Update the cache with the full task information.
	h.updateCache(task, root, env, hash, resolved)

	return nil
}

// updateCache updates the cache entry for a task and rebuilds the path-to-task index.
func (h *HashCache) updateCache(task *domain.Task, root string, env map[string]string, hash string, resolved []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cacheKey := h.makeCacheKey(task.Name.String(), root, env)
	taskHandle := task.Name.Value()
	pathHandles := make([]unique.Handle[string], len(resolved))

	// Remove old index entries for this cache key.
	h.removeTaskFromIndex(cacheKey)

	// Add new entry and build new index.
	for i, path := range resolved {
		pathHandle := unique.Make(path)
		pathHandles[i] = pathHandle

		// Add to path-to-task index (using cache key for invalidation).
		h.pathToTasks[pathHandle] = append(h.pathToTasks[pathHandle], cacheEntry{cacheKey: cacheKey})
	}

	// Store the cache entry.
	h.entries[cacheKey] = &domain.TaskHashEntry{
		Hash:           hash,
		ResolvedInputs: pathHandles,
		ComputedAt:     time.Now(),
	}

	// Store the cache key context for future invalidation.
	// Deep copy the env map to prevent shared reference bugs.
	h.cacheKeyContext[cacheKey] = PendingRehash{
		TaskName: task.Name.String(),
		Root:     root,
		Env:      copyEnv(env),
	}

	// Store the task definition (using simple task name handle).
	h.tasks[taskHandle] = task

	// Remove from pending rehashes if it was pending.
	if _, wasPending := h.pendingKeys[cacheKey]; wasPending {
		// O(1) removal from pending keys set
		delete(h.pendingKeys, cacheKey)

		// O(n) removal from pending list (needed for background worker iteration)
		for i, pending := range h.pendingRehashes {
			pendingKey := h.makeCacheKey(pending.TaskName, pending.Root, pending.Env)
			if pendingKey == cacheKey {
				h.pendingRehashes = append(h.pendingRehashes[:i], h.pendingRehashes[i+1:]...)
				break
			}
		}
	}
}

// removeTaskFromIndex removes all index entries for the given cache key.
func (h *HashCache) removeTaskFromIndex(cacheKey unique.Handle[string]) {
	for path, entries := range h.pathToTasks {
		for i, entry := range entries {
			if entry.cacheKey == cacheKey {
				// Remove this entry from the slice.
				h.pathToTasks[path] = append(entries[:i], entries[i+1:]...)
				if len(h.pathToTasks[path]) == 0 {
					// Delete empty entries.
					delete(h.pathToTasks, path)
				}
				break
			}
		}
	}
}

// GetPendingTasks returns a list of pending rehash entries with full context.
// This is used by the background worker to know which tasks to rehash.
func (h *HashCache) GetPendingTasks() []PendingRehash {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return a deep copy of the pending list to avoid exposing internal state.
	// Must deep-copy Env maps to prevent shared reference bugs.
	pending := make([]PendingRehash, len(h.pendingRehashes))
	for i, p := range h.pendingRehashes {
		pending[i] = PendingRehash{
			TaskName: p.TaskName,
			Root:     p.Root,
			Env:      copyEnv(p.Env),
		}
	}
	return pending
}
