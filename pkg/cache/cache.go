// Package cache provides a Redis-backed caching layer with singleflight
// deduplication for cache misses.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Cache is a simple read-through cache backed by Redis.
type Cache struct {
	rdb *redis.Client
	sf  singleflight.Group
}

// New creates a Cache backed by the given Redis client.
func New(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb}
}

// Get fetches the raw bytes for key from Redis. If the key is missing,
// it returns redis.Nil as the error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	if c.rdb == nil {
		return nil, redis.Nil
	}
	return c.rdb.Get(ctx, key).Bytes()
}

// Set stores val in Redis with the given TTL.
func (c *Cache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

// Delete removes key from Redis.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, key).Err()
}

// GetOrFetch attempts to fetch from cache; on miss it calls fetch once
// (using singleflight) and caches the result.
func (c *Cache) GetOrFetch(ctx context.Context, key string, ttl time.Duration, fetch func() ([]byte, error)) ([]byte, error) {
	if c.rdb == nil {
		return fetch()
	}

	val, err := c.Get(ctx, key)
	if err == nil {
		return val, nil
	}
	if err != redis.Nil {
		return nil, fmt.Errorf("cache get %s: %w", key, err)
	}

	v, err, _ := c.sf.Do(key, func() (any, error) {
		b, err := fetch()
		if err != nil {
			return nil, err
		}
		if err := c.Set(ctx, key, b, ttl); err != nil {
			// Log but don't fail the request if cache write fails.
			_ = err
		}
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}
