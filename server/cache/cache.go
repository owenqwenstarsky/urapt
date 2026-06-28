// Package cache defines the in-memory APT index cache contract used by the
// REST API (to invalidate on mutation) and the APT endpoint (to serve cached
// indices). The implementation lives in this package; it is regenerated lazily
// after invalidation or restart.
package cache

import (
	"sync"

	"urapt/shared/apt"
)

// IndexCache holds generated APT indices per (repository, suite), keyed and
// versioned so that mutations invalidate lazily.
type IndexCache struct {
	mu    sync.Mutex
	entry map[string]*entry
}

type entry struct {
	indices *apt.Indices
	dirty   bool
}

// New constructs an empty IndexCache.
func New() *IndexCache {
	return &IndexCache{entry: map[string]*entry{}}
}

// key builds the cache key.
func key(repoID, suite string) string { return repoID + "/" + suite }

// Get returns the cached indices for (repoID, suite), or nil if not present
// or marked dirty. The suite is returned so the caller can rebuild it.
func (c *IndexCache) Get(repoID, suite string) *apt.Indices {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entry[key(repoID, suite)]
	if e == nil || e.dirty {
		return nil
	}
	return e.indices
}

// Put stores freshly generated indices for (repoID, suite).
func (c *IndexCache) Put(repoID, suite string, idx *apt.Indices) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entry[key(repoID, suite)]
	if e == nil {
		e = &entry{}
		c.entry[key(repoID, suite)] = e
	}
	e.indices = idx
	e.dirty = false
}

// Invalidate marks one (repository, suite) as stale; the next read rebuilds.
func (c *IndexCache) Invalidate(repoID, suite string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entry[key(repoID, suite)]; e != nil {
		e.dirty = true
	}
}

// InvalidateRepo marks every suite under a repository as stale.
func (c *IndexCache) InvalidateRepo(repoID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := repoID + "/"
	for k, e := range c.entry {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			e.dirty = true
		}
	}
}
