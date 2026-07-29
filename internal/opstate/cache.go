package opstate

import (
	"sync"
	"time"
)

// Cache is a small TTL cache for parsed state sections, keyed by
// host|vendor|statetype. It exists so an agent can ask for operational state
// repeatedly (or for several state types in separate calls) without re-hitting
// the device every time; the caller passes force=true to bypass it for the
// volatile sections. A nil *Cache is a valid disabled cache.
type Cache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]cacheItem
}

type cacheItem struct {
	// section holds the parsed section snapshot. It is treated as immutable
	// once stored; readers copy the slice headers into their own State and only
	// read the underlying data, so sharing the pointer across requests is safe.
	section *State
	exp     time.Time
}

// NewCache returns a cache with the given TTL. A ttl <= 0 disables caching
// (every lookup misses).
func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, items: map[string]cacheItem{}}
}

func (c *Cache) get(key string) (*State, bool) {
	if c == nil || c.ttl <= 0 {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	it, ok := c.items[key]
	if !ok || time.Now().After(it.exp) {
		return nil, false
	}
	return it.section, true
}

func (c *Cache) set(key string, section *State) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cacheItem{section: section, exp: time.Now().Add(c.ttl)}
}
