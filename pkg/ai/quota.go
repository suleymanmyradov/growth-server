package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// QuotaStore is the interface for per-user and global quota tracking.
// Implementations must be safe for concurrent use.
type QuotaStore interface {
	// CheckUserQuota returns false if the user has exceeded their daily token cap.
	CheckUserQuota(ctx context.Context, userID string, cap int64) (ok bool, err error)
	// IncrUserTokens increments the user's daily token count.
	IncrUserTokens(ctx context.Context, userID string, tokens int64) error
	// CheckGlobalQuota returns false if the global daily cost cap is exceeded.
	// costCap is in microdollars (1e-6 USD).
	CheckGlobalQuota(ctx context.Context, costCap int64) (ok bool, err error)
	// IncrGlobalCost increments the global daily cost in microdollars.
	IncrGlobalCost(ctx context.Context, microDollars int64) error
}

// redisQuotaStore implements QuotaStore using Redis.
type redisQuotaStore struct {
	client *redis.Client
}

// NewRedisQuotaStore creates a QuotaStore backed by Redis.
func NewRedisQuotaStore(client *redis.Client) QuotaStore {
	return &redisQuotaStore{client: client}
}

func dailyKey(prefix, id string) string {
	return fmt.Sprintf("ai:quota:%s:%s:%s", prefix, id, time.Now().UTC().Format("2006-01-02"))
}

func (s *redisQuotaStore) CheckUserQuota(ctx context.Context, userID string, cap int64) (bool, error) {
	key := dailyKey("user", userID)
	val, err := s.client.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("ai.QuotaStore: get user quota: %w", err)
	}
	return val < cap, nil
}

func (s *redisQuotaStore) IncrUserTokens(ctx context.Context, userID string, tokens int64) error {
	key := dailyKey("user", userID)
	if err := s.client.IncrBy(ctx, key, tokens).Err(); err != nil {
		return fmt.Errorf("ai.QuotaStore: incr user tokens: %w", err)
	}
	// Set TTL to 48h on first write (covers timezone edge).
	s.client.Expire(ctx, key, 48*time.Hour)
	return nil
}

func (s *redisQuotaStore) CheckGlobalQuota(ctx context.Context, costCap int64) (bool, error) {
	key := dailyKey("global", "cost")
	val, err := s.client.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("ai.QuotaStore: get global quota: %w", err)
	}
	return val < costCap, nil
}

func (s *redisQuotaStore) IncrGlobalCost(ctx context.Context, microDollars int64) error {
	key := dailyKey("global", "cost")
	if err := s.client.IncrBy(ctx, key, microDollars).Err(); err != nil {
		return fmt.Errorf("ai.QuotaStore: incr global cost: %w", err)
	}
	s.client.Expire(ctx, key, 48*time.Hour)
	return nil
}

// noopQuotaStore is a no-op implementation used when quota is disabled.
type noopQuotaStore struct{}

func (noopQuotaStore) CheckUserQuota(_ context.Context, _ string, _ int64) (bool, error) {
	return true, nil
}
func (noopQuotaStore) IncrUserTokens(_ context.Context, _ string, _ int64) error { return nil }
func (noopQuotaStore) CheckGlobalQuota(_ context.Context, _ int64) (bool, error) { return true, nil }
func (noopQuotaStore) IncrGlobalCost(_ context.Context, _ int64) error           { return nil }

// Ensure redisQuotaStore implements QuotaStore.
var _ QuotaStore = (*redisQuotaStore)(nil)
