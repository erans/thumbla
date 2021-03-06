package cache

import (
	lru "github.com/hashicorp/golang-lru"

	"github.com/erans/thumbla/config"
)

// InMemoryCache providers a generic in-memory LRU cache
type InMemoryCache struct {
	Cache *lru.Cache
}

// Contains checks if a key exists in the cache
func (m *InMemoryCache) Contains(key string) bool {
	return m.Cache.Contains(key)
}

// Get returns a cached item, if it exists, otherwise returns nil
func (m *InMemoryCache) Get(key string) interface{} {
	if v, ok := m.Cache.Get(key); ok {
		return v
	}
	return nil
}

// Set saves an item into the cache
func (m *InMemoryCache) Set(key string, value interface{}) {
	m.Cache.Add(key, value)
}

// Clear cleans the cache
func (m *InMemoryCache) Clear() {
	m.Cache.Purge()
}

// NewInMemoryCache returns a new instance of the in-memory LRU based cache
func NewInMemoryCache(cfg *config.Config) *InMemoryCache {
	if newCache, err := lru.New(cfg.Cache.InMemory.Size); err == nil {
		return &InMemoryCache{
			Cache: newCache,
		}
	}

	return nil
}
