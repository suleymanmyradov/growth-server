package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
)

// Redis-backed RevocationRepository, shared by every service that issues or
// verifies revocable tokens (auth RPC, adminway). Keys are namespaced by the
// prefixes below; entries expire with the token's own lifetime.

const (
	revokedAccessPrefix  = "revoked:access:"
	revokedRefreshPrefix = "revoked:refresh:"
	revokedSessionPrefix = "revoked:session:"
	minRedisTTL          = 100 * time.Millisecond
)

// RedisRevocationRepository implements RevocationRepository on top of any
// redis.Cmdable (standalone client, cluster client, ...).
type RedisRevocationRepository struct {
	client redis.Cmdable
}

// NewRedisRevocationRepository returns a Redis-backed revocation repository.
func NewRedisRevocationRepository(client redis.Cmdable) (RevocationRepository, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	return &RedisRevocationRepository{
		client: client,
	}, nil
}

func (r *RedisRevocationRepository) MarkTokenRevoke(ctx context.Context, tokenType TokenType, token string, ttl time.Duration) error {
	if ttl < minRedisTTL {
		ttl = minRedisTTL
	}

	var key string
	switch tokenType {
	case AccessToken:
		key = revokedAccessPrefix + token
	case RefreshToken:
		key = revokedRefreshPrefix + token
	default:
		return fmt.Errorf("invalid token type: %v", tokenType)
	}

	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return r.client.Set(ctx, key, "1", ttl).Err()
}

func (r *RedisRevocationRepository) IsTokenRevoked(ctx context.Context, tokenType TokenType, token string) (bool, error) {
	var key string
	switch tokenType {
	case AccessToken:
		key = revokedAccessPrefix + token
	case RefreshToken:
		key = revokedRefreshPrefix + token
	default:
		return false, fmt.Errorf("invalid token type: %v", tokenType)
	}

	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check revocation: %w", err)
	}

	return exists > 0, nil
}

func (r *RedisRevocationRepository) MarkSessionRevoked(ctx context.Context, sessionID string, ttl time.Duration) error {
	if ttl < minRedisTTL {
		ttl = minRedisTTL
	}

	key := revokedSessionPrefix + sessionID
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return r.client.Set(ctx, key, "1", ttl).Err()
}

func (r *RedisRevocationRepository) IsSessionRevoked(ctx context.Context, sessionID string) (bool, error) {
	key := revokedSessionPrefix + sessionID
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check session revocation: %w", err)
	}
	return exists > 0, nil
}
