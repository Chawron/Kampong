package search

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kampong/debate/internal/models"
)

// ResearchCache stores search results to avoid duplicate API calls.
type ResearchCache struct {
	mu        sync.RWMutex
	entries   map[string]*models.ResearchCacheEntry
	ttl       time.Duration
	totalHits atomic.Int64

	// Optional hit/miss sink so the API layer can pipe counters into /api/metrics
	// without this package importing the API package (circular dep).
	onHit    func()
	onMiss   func()
}

// SetSink installs hit/miss callbacks. Both may be nil.
func (c *ResearchCache) SetSink(onHit, onMiss func()) {
	c.onHit = onHit
	c.onMiss = onMiss
}

// NewResearchCache creates a cache with the given TTL for entries.
func NewResearchCache(ttl time.Duration) *ResearchCache {
	return &ResearchCache{
		entries: make(map[string]*models.ResearchCacheEntry),
		ttl:     ttl,
	}
}

// normalizeKey lowercases and trims the query for use as a cache key.
func normalizeKey(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

// Get returns cached results if they exist and are still fresh.
func (c *ResearchCache) Get(query string) ([]models.SearchResult, bool) {
	key := normalizeKey(query)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		if c.onMiss != nil {
			c.onMiss()
		}
		return nil, false
	}

	// Check TTL
	if time.Since(entry.Timestamp) > c.ttl {
		// Expired — remove it
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return nil, false
	}

	// Fresh hit — increment hit count
	c.mu.Lock()
	entry.HitCount++
	c.mu.Unlock()

	c.totalHits.Add(1)
	if c.onHit != nil {
		c.onHit()
	}

	// Return a copy so callers can't mutate the cache
	out := make([]models.SearchResult, len(entry.Results))
	copy(out, entry.Results)
	return out, true
}

// Set stores search results in the cache.
func (c *ResearchCache) Set(query string, results []models.SearchResult) {
	key := normalizeKey(query)

	stored := make([]models.SearchResult, len(results))
	copy(stored, results)

	c.mu.Lock()
	c.entries[key] = &models.ResearchCacheEntry{
		Query:     query,
		Results:   stored,
		Timestamp: time.Now(),
		HitCount:  0,
	}
	c.mu.Unlock()
}

// HitCount returns the total number of cache hits across all queries.
func (c *ResearchCache) HitCount() int {
	return int(c.totalHits.Load())
}
