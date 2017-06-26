package cache

import "github.com/erans/thumbla/config"

// DummyCache provider an empty cache implementation that caches nothing!
type DummyCache struct {
}

// Contains checks if a key exists in the cache
func (m *DummyCache) Contains(key string) bool {
	return false
}

// Get returns a cached item, if it exists, otherwise returns nil
func (m *DummyCache) Get(key string) interface{} {
	return nil
}

// Set saves an item into the cache
func (m *DummyCache) Set(key string, value interface{}) {
}

// Clear cleans the cache
func (m *DummyCache) Clear() {
}

// NewDummyCache returns a new dummy cache that does nothing
func NewDummyCache(cfg *config.Config) *DummyCache {
	return &DummyCache{}
}
