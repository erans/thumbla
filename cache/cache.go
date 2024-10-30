package cache

import "github.com/erans/thumbla/config"

const (
	// CacheDummy configuration key
	CacheDummy = "dummy"
	// CacheInMemory configuration key
	CacheInMemory = "inmemory"
)

// Cache is a simple interface to interact with different cache mechanisms supported on thumbla
type Cache interface {
	Contains(key string) bool
	Get(key string) interface{}
	Set(key string, value interface{})
	Clear()
}

var globalCache Cache

var cacheRegistry map[string]Cache

// InitCache returns the currently active cache
func InitCache(cfg *config.Config) {
	if len(cacheRegistry) == 0 {
		cacheRegistry = map[string]Cache{
			CacheDummy:    NewDummyCache(cfg),
			CacheInMemory: NewInMemoryCache(cfg),
			CacheRedis:    NewRedisCache(cfg),
		}
	}

	if cfg.Cache.Provider == "" {
		cfg.Cache.Provider = CacheDummy
	}

	if tempCache, ok := cacheRegistry[cfg.Cache.Provider]; ok {
		globalCache = tempCache
	}
}

// GetCache returns the currently active cache
func GetCache() Cache {
	return globalCache
}
