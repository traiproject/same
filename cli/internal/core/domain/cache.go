// Package domain contains core domain types for caching.
package domain

// GraphCacheEntry holds a cached graph with its validation metadata.
type GraphCacheEntry struct {
	Graph       *Graph
	ConfigPaths []string         // Paths to same.yaml / same.work.yaml
	Mtimes      map[string]int64 // path -> mtime in UnixNano
}
