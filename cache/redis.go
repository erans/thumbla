package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/erans/thumbla/config"
	"github.com/redis/go-redis/v9"
)

const (
	// CacheRedis configuration key
	CacheRedis = "redis"
)

// RedisCache provides Redis-backed LRU cache implementation
type RedisCache struct {
	client     *redis.Client
	ctx        context.Context
	maxLRUSize int
}

// Contains checks if a key exists in the Redis cache
func (r *RedisCache) Contains(key string) bool {
	exists, err := r.client.Exists(r.ctx, key).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

// Get returns a cached item from Redis if it exists
func (r *RedisCache) Get(key string) interface{} {
	val, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		return nil
	}

	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil
	}

	// Update access time to maintain LRU order
	r.client.ZIncrBy(r.ctx, "cache_access", 1, key)

	return result
}

// Set saves an item into Redis cache
func (r *RedisCache) Set(key string, value interface{}) {
	// Convert value to JSON for storage
	jsonVal, err := json.Marshal(value)
	if err != nil {
		return
	}

	// Store the value
	r.client.Set(r.ctx, key, jsonVal, 0)

	// Add/update access count for LRU
	r.client.ZAdd(r.ctx, "cache_access", redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: key,
	})

	// Only trim if maxLRUSize is not -1 (unlimited)
	if r.maxLRUSize != -1 {
		// Trim to max size if needed
		count, err := r.client.ZCard(r.ctx, "cache_access").Result()
		if err == nil && count > 0 {
			// Remove oldest entries if we exceed cache size
			if count > int64(r.maxLRUSize) {
				// Calculate how many items to remove
				toRemove := count - int64(r.maxLRUSize)
				r.client.ZPopMin(r.ctx, "cache_access", toRemove)
			}
		}
	}
}

// Clear removes all items from the Redis cache
func (r *RedisCache) Clear() {
	r.client.FlushDB(r.ctx)
}

// NewRedisCache returns a new Redis cache instance
func NewRedisCache(cfg *config.Config) *RedisCache {
	ctx := context.Background()

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Cache.Redis.Address,
		Password: cfg.Cache.Redis.Password,
		DB:       cfg.Cache.Redis.DB,
	})

	return &RedisCache{
		client:     client,
		ctx:        ctx,
		maxLRUSize: cfg.Cache.Redis.MaxLRUSize,
	}
}
