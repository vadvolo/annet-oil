package cache

import (
	"testing"

	"annet-oil/internal/config"
)

func newTestCache(maxSize string) *MemoryCache {
	return New(config.CacheConfig{Enabled: true, TTL: "1m", MaxSize: maxSize})
}

func TestCacheGetSet(t *testing.T) {
	c := newTestCache("1MB")
	c.Set("k1", []byte("hello"))

	if v, ok := c.Get("k1"); !ok || string(v) != "hello" {
		t.Errorf("get k1 = %q, %v", v, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Error("expected miss for absent key")
	}

	c.Delete("k1")
	if _, ok := c.Get("k1"); ok {
		t.Error("expected miss after delete")
	}
}

// TestCacheEvictionUntilUnderLimit verifies Set evicts as many old entries as
// needed to fit the new value, not just one.
func TestCacheEvictionUntilUnderLimit(t *testing.T) {
	// maxSize 100 bytes; three 40-byte values cannot coexist.
	c := newTestCache("100")
	val := make([]byte, 40)

	c.Set("a", val)
	c.Set("b", val)
	c.Set("c", val) // must evict until under 100 bytes

	if c.size > c.maxSize {
		t.Errorf("cache size %d exceeds maxSize %d after eviction", c.size, c.maxSize)
	}

	// The most recent key must still be present.
	if _, ok := c.Get("c"); !ok {
		t.Error("most recently set key 'c' should be present")
	}
}

func TestCacheDisabled(t *testing.T) {
	c := New(config.CacheConfig{Enabled: false})
	c.Set("k", []byte("v"))
	if _, ok := c.Get("k"); ok {
		t.Error("disabled cache should not store values")
	}
}
