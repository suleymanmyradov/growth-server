package cache

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	validAccessTokensKey  = "auth:valid_access_tokens"
	swapTokensKeyPrefix   = "auth:swap_token:"
	swapTokenActiveStatus = "active"
)

type Cache interface {
	IsValidAccessToken(ctx context.Context, accessToken string) (bool, error)
	AddToValidTokens(ctx context.Context, accessToken string) error
	RemoveFromValidTokens(ctx context.Context, accessToken string) error
	AddTokenToSwappableTokens(ctx context.Context, accessToken string, ttlSeconds int) error
	IsSwappableToken(ctx context.Context, accessToken string) (bool, error)
}

type cache struct {
	client *redis.Redis
}

func NewCache(client *redis.Redis) Cache {
	return &cache{client: client}
}

func (c *cache) IsValidAccessToken(ctx context.Context, accessToken string) (bool, error) {
	return c.client.SismemberCtx(ctx, validAccessTokensKey, accessToken)
}

func (c *cache) AddToValidTokens(ctx context.Context, accessToken string) error {
	_, err := c.client.SaddCtx(ctx, validAccessTokensKey, accessToken)
	return err
}

func (c *cache) RemoveFromValidTokens(ctx context.Context, accessToken string) error {
	_, err := c.client.SremCtx(ctx, validAccessTokensKey, accessToken)
	return err
}

func (c *cache) AddTokenToSwappableTokens(ctx context.Context, accessToken string, ttlSeconds int) error {
	key := swapTokensKeyPrefix + accessToken
	return c.client.SetexCtx(ctx, key, swapTokenActiveStatus, ttlSeconds)
}

func (c *cache) IsSwappableToken(ctx context.Context, accessToken string) (bool, error) {
	key := swapTokensKeyPrefix + accessToken
	status, err := c.client.GetCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("IsSwappableToken.Get - error: %v", err)
		return false, nil
	}

	return status == swapTokenActiveStatus, nil
}
