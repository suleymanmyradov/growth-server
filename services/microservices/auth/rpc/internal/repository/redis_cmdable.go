package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
)

const (
	revokedAccessPrefix  = "revoked:access:"
	revokedRefreshPrefix = "revoked:refresh:"
	minRedisTTL          = 100 * time.Millisecond
)

type CmdableRedisRepository struct {
	client redis.Cmdable
}

func NewCmdableRedisRepository(client redis.Cmdable) (jwt.RevocationRepository, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	return &CmdableRedisRepository{
		client: client,
	}, nil
}

func (r *CmdableRedisRepository) MarkTokenRevoke(ctx context.Context, tokenType jwt.TokenType, token string, ttl time.Duration) error {
	if ttl < minRedisTTL {
		ttl = minRedisTTL
	}

	var key string
	switch tokenType {
	case jwt.AccessToken:
		key = revokedAccessPrefix + token
	case jwt.RefreshToken:
		key = revokedRefreshPrefix + token
	default:
		return fmt.Errorf("invalid token type: %v", tokenType)
	}

	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return r.client.Set(ctx, key, "1", ttl).Err()
}

func (r *CmdableRedisRepository) IsTokenRevoked(ctx context.Context, tokenType jwt.TokenType, token string) (bool, error) {
	var key string
	switch tokenType {
	case jwt.AccessToken:
		key = revokedAccessPrefix + token
	case jwt.RefreshToken:
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
