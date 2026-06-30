package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
)

const emailVerificationPrefix = "auth:email-verification:"

// VerificationEntry holds the stored state for an email verification token.
type VerificationEntry struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

// VerificationRepo stores email verification tokens in Redis with TTL.
type VerificationRepo struct {
	client redis.Cmdable
}

func NewVerificationRepo(client redis.Cmdable) *VerificationRepo {
	return &VerificationRepo{client: client}
}

// Store saves a verification token with the given TTL.
func (r *VerificationRepo) Store(ctx context.Context, token, userID, email string, ttl time.Duration) error {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	entry := VerificationEntry{UserID: userID, Email: email, ExpiresAt: time.Now().Add(ttl)}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal verification entry: %w", err)
	}
	key := emailVerificationPrefix + token
	return r.client.Set(ctx, key, data, ttl).Err()
}

// Get retrieves and validates a verification token. If expired or missing, ok is false.
func (r *VerificationRepo) Get(ctx context.Context, token string) (entry VerificationEntry, ok bool, err error) {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	key := emailVerificationPrefix + token
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return VerificationEntry{}, false, nil
	}
	if err != nil {
		return VerificationEntry{}, false, fmt.Errorf("redis get: %w", err)
	}

	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return VerificationEntry{}, false, fmt.Errorf("unmarshal verification entry: %w", err)
	}

	if time.Now().After(entry.ExpiresAt) {
		_ = r.client.Del(ctx, key).Err()
		return VerificationEntry{}, false, nil
	}

	return entry, true, nil
}

// Delete removes a verification token from Redis.
func (r *VerificationRepo) Delete(ctx context.Context, token string) error {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	key := emailVerificationPrefix + token
	return r.client.Del(ctx, key).Err()
}

// Exists reports whether a verification token is currently stored for the given email.
// Used for resend rate-limiting: a non-empty result means a valid token is already pending.
func (r *VerificationRepo) PendingForEmail(ctx context.Context, email string) (bool, error) {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	// We store tokens keyed by random token, not by email. For rate-limiting we use a
	// separate short-lived marker key so resends are throttled (e.g. 60s).
	key := emailVerificationPrefix + "throttle:" + email
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return n > 0, nil
}

// SetThrottle marks the email as having a pending resend window for the given TTL.
func (r *VerificationRepo) SetThrottle(ctx context.Context, email string, ttl time.Duration) error {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	key := emailVerificationPrefix + "throttle:" + email
	return r.client.Set(ctx, key, "1", ttl).Err()
}
