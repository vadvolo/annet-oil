package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"annet-oil/internal/config"
	"annet-oil/internal/logging"
)

type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte)
	Delete(key string)
	InvalidateAll()
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]entry
	ttl     time.Duration
	maxSize int64
	size    int64
	enabled bool
}

func New(cfg config.CacheConfig) *MemoryCache {
	ttl := parseDuration(cfg.TTL, 5*time.Minute)
	maxSize := parseSize(cfg.MaxSize, 100*1024*1024)

	c := &MemoryCache{
		items:   make(map[string]entry),
		ttl:     ttl,
		maxSize: maxSize,
		enabled: cfg.Enabled,
	}

	go c.cleanup()

	return c
}

func (c *MemoryCache) Get(key string) ([]byte, bool) {
	if !c.enabled {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.items[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(e.expiresAt) {
		return nil, false
	}

	logging.Debug("cache hit", "key", keyPrefix(key))
	return e.value, true
}

func (c *MemoryCache) Set(key string, value []byte) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entries until the new value fits, rather than evicting a
	// single entry that may not free enough space.
	for c.size+int64(len(value)) > c.maxSize && len(c.items) > 0 {
		c.evictOldest()
	}

	c.items[key] = entry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.size += int64(len(value))

	logging.Debug("cache set", "key", keyPrefix(key), "size", len(value))
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.items[key]; ok {
		c.size -= int64(len(e.value))
		delete(c.items, key)
	}
}

func (c *MemoryCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]entry)
	c.size = 0

	logging.Info("cache invalidated")
}

func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// Collect expired keys under a read lock so Get/Set aren't blocked for
		// the whole scan, then delete them under a short write lock.
		var expired []string
		c.mu.RLock()
		for key, e := range c.items {
			if now.After(e.expiresAt) {
				expired = append(expired, key)
			}
		}
		c.mu.RUnlock()

		if len(expired) == 0 {
			continue
		}

		c.mu.Lock()
		for _, key := range expired {
			// Re-check under the write lock in case the entry was refreshed.
			if e, ok := c.items[key]; ok && now.After(e.expiresAt) {
				c.size -= int64(len(e.value))
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, e := range c.items {
		if oldestKey == "" || e.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = e.expiresAt
		}
	}

	if oldestKey != "" {
		c.size -= int64(len(c.items[oldestKey].value))
		delete(c.items, oldestKey)
	}
}

// keyPrefix returns the first 16 chars of a key for debug logging, guarding
// against keys shorter than 16 (production keys are 64-char sha256 hex).
func keyPrefix(key string) string {
	if len(key) > 16 {
		return key[:16]
	}
	return key
}

func HashRequest(v any) string {
	data, _ := json.Marshal(v)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func parseSize(s string, def int64) int64 {
	if s == "" {
		return def
	}

	multiplier := int64(1)
	if len(s) >= 2 {
		suffix := s[len(s)-2:]
		switch suffix {
		case "KB", "kb":
			multiplier = 1024
			s = s[:len(s)-2]
		case "MB", "mb":
			multiplier = 1024 * 1024
			s = s[:len(s)-2]
		case "GB", "gb":
			multiplier = 1024 * 1024 * 1024
			s = s[:len(s)-2]
		}
	}

	var size int64
	if err := json.Unmarshal([]byte(s), &size); err != nil {
		return def
	}

	return size * multiplier
}
