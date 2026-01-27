// Package daemon provides the daemon server and client implementations.
package daemon

import (
	"sync"

	"go.trai.ch/same/internal/core/domain"
)

// ServerCache holds thread-safe in-memory caches for the daemon server.
//
// Cache Validation Assumption:
// The cache validation logic trusts client-provided mtime values and compares them
// against stored mtimes without verifying actual file mtimes on the server.
// This design assumes the daemon and client share the same filesystem view, which
// is valid for local Unix-socket daemons but would need revision for remote scenarios.
type ServerCache struct {
	mu         sync.RWMutex
	graphCache map[string]*domain.GraphCacheEntry // cwd -> entry
	envCache   map[string][]string                // envID -> env vars
}

// NewServerCache creates a new ServerCache instance.
func NewServerCache() *ServerCache {
	return &ServerCache{
		graphCache: make(map[string]*domain.GraphCacheEntry),
		envCache:   make(map[string][]string),
	}
}

// GetGraph retrieves a cached graph for the given cwd and validates mtimes.
// Returns the graph and true if cache hit and valid, nil and false otherwise.
func (c *ServerCache) GetGraph(cwd string, clientMtimes map[string]int64) (*domain.Graph, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.graphCache[cwd]
	if !exists {
		return nil, false
	}

	// Validate mtimes match
	if len(clientMtimes) != len(entry.Mtimes) {
		return nil, false
	}

	for path, clientMtime := range clientMtimes {
		storedMtime, ok := entry.Mtimes[path]
		if !ok || clientMtime != storedMtime {
			return nil, false
		}
	}

	return entry.Graph, true
}

// SetGraph stores a graph in the cache with its mtimes.
func (c *ServerCache) SetGraph(cwd string, entry *domain.GraphCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.graphCache[cwd] = entry
}

// GetEnv retrieves cached environment variables for the given envID.
// Returns the env vars and true if cache hit, nil and false otherwise.
func (c *ServerCache) GetEnv(envID string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	env, exists := c.envCache[envID]
	return env, exists
}

// SetEnv stores environment variables in the cache.
func (c *ServerCache) SetEnv(envID string, env []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.envCache[envID] = env
}
