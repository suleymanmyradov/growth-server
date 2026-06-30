package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
)

const passwordResetPrefix = "auth:password-reset:"

// PasswordResetEntry holds the stored state for a password reset token.
type PasswordResetEntry struct {
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PasswordResetRepo stores password reset tokens in Redis with TTL.
type PasswordResetRepo struct {
	client redis.Cmdable
}

// NewPasswordResetRepo creates a new PasswordResetRepo backed by Redis.
func NewPasswordResetRepo(client redis.Cmdable) *PasswordResetRepo {
	return &PasswordResetRepo{client: client}
}

// Store saves a password reset token with the given TTL.
func (r *PasswordResetRepo) Store(ctx context.Context, token, email string, ttl time.Duration) error {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	entry := PasswordResetEntry{Email: email, ExpiresAt: time.Now().Add(ttl)}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal password reset entry: %w", err)
	}
	key := passwordResetPrefix + token
	return r.client.Set(ctx, key, data, ttl).Err()
}

// Get retrieves and validates a password reset token. If expired or missing, ok is false.
func (r *PasswordResetRepo) Get(ctx context.Context, token string) (entry PasswordResetEntry, ok bool, err error) {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	key := passwordResetPrefix + token
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return PasswordResetEntry{}, false, nil
	}
	if err != nil {
		return PasswordResetEntry{}, false, fmt.Errorf("redis get: %w", err)
	}

	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return PasswordResetEntry{}, false, fmt.Errorf("unmarshal password reset entry: %w", err)
	}

	if time.Now().After(entry.ExpiresAt) {
		_ = r.client.Del(ctx, key).Err()
		return PasswordResetEntry{}, false, nil
	}

	return entry, true, nil
}

// Delete removes a password reset token from Redis.
func (r *PasswordResetRepo) Delete(ctx context.Context, token string) error {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	key := passwordResetPrefix + token
	return r.client.Del(ctx, key).Err()
}
