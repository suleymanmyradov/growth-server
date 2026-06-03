// Package redisutil provides a resilient Redis client factory and helpers
// used across all backend services.
package redisutil

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultOpts returns tuned redis.Options suitable for production use.
// Callers can override specific fields after receiving the defaults.
func DefaultOpts(addr, password string, db int) *redis.Options {
	return &redis.Options{
		Addr:            addr,
		Password:        password,
		DB:              db,
		PoolSize:        20,
		MinIdleConns:    5,
		DialTimeout:     2 * time.Second,
		ReadTimeout:     1 * time.Second,
		WriteTimeout:    1 * time.Second,
		PoolTimeout:     2 * time.Second,
		ConnMaxIdleTime: 10 * time.Minute,
	}
}

// NewClient creates a go-redis client with production-ready defaults and
// verifies connectivity with a Ping. Returns an error instead of panicking
// so callers can decide whether to fail or degrade gracefully.
func NewClient(addr, password string, db int) (*redis.Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("redisutil: addr is required")
	}
	c := redis.NewClient(DefaultOpts(addr, password, db))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redisutil: ping failed: %w", err)
	}
	return c, nil
}

// JitteredTTL returns a TTL with random jitter added to prevent
// synchronized mass expiry. Safe for concurrent use.
//
// Example: JitteredTTL(60*time.Second, 10*time.Second) returns 60s-70s.
func JitteredTTL(base, maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 {
		return base
	}
	jitter := time.Duration(rand.Int63n(int64(maxJitter)))
	return base + jitter
}

// WithTimeout returns a child context with the given timeout, or the original
// context if timeout is zero. The caller is responsible for calling cancel.
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, timeout)
}
