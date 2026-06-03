// Package authz provides lightweight authorization helpers for downstream services.
// It caches user existence/active status in Redis to avoid repeated DB lookups.
package authz

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	userStatusPrefix = "authz:user-status:"
	userStatusTTL    = 60 * time.Second
)

// Cache is the minimal interface required by Checker for Redis operations.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisCache adapts a redis.Cmdable to the Cache interface.
type RedisCache struct {
	client interface {
		Get(ctx context.Context, key string) *redis.StringCmd
		Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd
		Del(ctx context.Context, keys ...string) *redis.IntCmd
	}
}

// NewRedisCache wraps a redis.Cmdable as a Cache.
func NewRedisCache(client redis.Cmdable) Cache {
	return &RedisCache{client: client}
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// UserStatus describes the cached authorization state for a user.
type UserStatus int

const (
	StatusUnknown UserStatus = iota
	StatusActive
	StatusInactive
	StatusNotFound
)

// Checker performs cached user status checks.
type Checker struct {
	cache Cache
	// Lookup is called on cache miss to fetch the canonical status from the DB.
	Lookup func(ctx context.Context, userID uuid.UUID) (UserStatus, error)
	group  singleflight.Group
}

// NewChecker creates an authz checker backed by the given cache.
func NewChecker(cache Cache, lookup func(ctx context.Context, userID uuid.UUID) (UserStatus, error)) *Checker {
	return &Checker{cache: cache, Lookup: lookup}
}

// CheckPrincipal verifies that the principal in ctx refers to an active user.
// It returns a gRPC Unauthenticated error if the principal is missing or the user is not active.
func (c *Checker) CheckPrincipal(ctx context.Context) error {
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return status.Error(codes.Unauthenticated, "invalid principal")
	}
	return c.CheckUser(ctx, userID)
}

// CheckUser verifies that the given userID is active, using the cache.
func (c *Checker) CheckUser(ctx context.Context, userID uuid.UUID) error {
	statusCode, err := c.getStatus(ctx, userID)
	if err != nil {
		return status.Error(codes.Internal, "authorization check failed")
	}
	switch statusCode {
	case StatusActive:
		return nil
	case StatusInactive:
		return status.Error(codes.PermissionDenied, "user account is inactive")
	case StatusNotFound:
		return status.Error(codes.Unauthenticated, "user not found")
	default:
		return status.Error(codes.Internal, "authorization check failed")
	}
}

// Invalidate removes the cached status for a user (call after updates that affect authz).
func (c *Checker) Invalidate(ctx context.Context, userID uuid.UUID) error {
	key := userStatusPrefix + userID.String()
	return c.cache.Del(ctx, key)
}

func (c *Checker) getStatus(ctx context.Context, userID uuid.UUID) (UserStatus, error) {
	key := userStatusPrefix + userID.String()
	val, err := c.cache.Get(ctx, key)
	if err == nil {
		switch val {
		case "active":
			return StatusActive, nil
		case "inactive":
			return StatusInactive, nil
		case "notfound":
			return StatusNotFound, nil
		}
	}
	// Cache miss or unexpected value; perform lookup with singleflight deduplication.
	result, err, _ := c.group.Do(key, func() (interface{}, error) {
		statusCode, lookupErr := c.Lookup(ctx, userID)
		if lookupErr != nil {
			return StatusUnknown, lookupErr
		}

		var cacheVal string
		switch statusCode {
		case StatusActive:
			cacheVal = "active"
		case StatusInactive:
			cacheVal = "inactive"
		case StatusNotFound:
			cacheVal = "notfound"
		default:
			return statusCode, nil
		}

		_ = c.cache.Set(ctx, key, cacheVal, userStatusTTL)
		return statusCode, nil
	})
	if err != nil {
		return StatusUnknown, err
	}
	return result.(UserStatus), nil
}

// MustParseUUID is a helper that parses a UUID or returns an error.
func MustParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// GRPCError converts a UserStatus into a gRPC status error.
func GRPCError(s UserStatus) error {
	switch s {
	case StatusActive:
		return nil
	case StatusInactive:
		return status.Error(codes.PermissionDenied, "user account is inactive")
	case StatusNotFound:
		return status.Error(codes.Unauthenticated, "user not found")
	default:
		return status.Error(codes.Internal, fmt.Sprintf("unknown user status: %d", s))
	}
}
