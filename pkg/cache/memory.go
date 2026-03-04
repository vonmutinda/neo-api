package cache

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*memoryEntry
}

type memoryEntry struct {
	value     []byte
	expiresAt time.Time
	counter   int64
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{items: make(map[string]*memoryEntry)}
}

func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.items[key] = &memoryEntry{value: value, expiresAt: exp}
	return nil
}

func (c *MemoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

func (c *MemoryCache) DeleteByPrefix(_ context.Context, prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.items {
		if strings.HasPrefix(key, prefix) {
			delete(c.items, key)
		}
	}
	return nil
}

func (c *MemoryCache) Increment(_ context.Context, key string, ttl time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[key]
	if !ok || (!entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt)) {
		var exp time.Time
		if ttl > 0 {
			exp = time.Now().Add(ttl)
		}
		c.items[key] = &memoryEntry{counter: 1, expiresAt: exp}
		return 1, nil
	}
	val := atomic.AddInt64(&entry.counter, 1)
	return val, nil
}

func (c *MemoryCache) Ping(_ context.Context) error { return nil }
func (c *MemoryCache) Close() error                 { return nil }

var _ Cache = (*MemoryCache)(nil)
