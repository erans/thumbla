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

// GetBytes returns the raw bytes stored for key.
func (r *RedisCache) GetBytes(key string) ([]byte, bool) {
	val, err := r.client.Get(r.ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	r.client.ZIncrBy(r.ctx, "cache_access", 1, key)
	return val, true
}

// SetBytes stores raw bytes with the configured TTL.
func (r *RedisCache) SetBytes(key string, value []byte) {
	ttl := time.Duration(config.GetConfig().GetRedisTTL()) * time.Second
	r.client.Set(r.ctx, key, value, ttl)
	r.client.ZAdd(r.ctx, "cache_access", redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: key,
	})
	r.enforceMaxSize()
}

// Set marshals value to JSON and delegates to SetBytes.
func (r *RedisCache) Set(key string, value interface{}) {
	jsonVal, err := json.Marshal(value)
	if err != nil {
		return
	}
	r.SetBytes(key, jsonVal)
}

// enforceMaxSize trims the LRU set and deletes the corresponding data keys.
func (r *RedisCache) enforceMaxSize() {
	maxSize := r.maxLRUSize
	if maxSize <= 0 {
		maxSize = 10000
	}
	count, err := r.client.ZCard(r.ctx, "cache_access").Result()
	if err != nil || count <= int64(maxSize) {
		return
	}
	toRemove := count - int64(maxSize)
	members, err := r.client.ZPopMin(r.ctx, "cache_access", toRemove).Result()
	if err != nil {
		return
	}
	keys := make([]string, 0, len(members))
	for _, z := range members {
		if s, ok := z.Member.(string); ok {
			keys = append(keys, s)
		}
	}
	if len(keys) > 0 {
		r.client.Del(r.ctx, keys...)
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
