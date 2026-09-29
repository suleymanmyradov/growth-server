package jwt

import (
	"context"
	"fmt"
	"strconv"
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
	revokedUserPrefix    = "revoked:user:"
	rotatedRefreshPrefix = "rotated:refresh:"
	minRedisTTL          = 100 * time.Millisecond
)

// unconsumeScript deletes a consumed-refresh marker only if its value still
// equals the caller's marker — the CAS half of ConsumeRefreshToken used when
// the follow-up store of the rotated pair fails.
var unconsumeScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`)

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

// ConsumeRefreshToken atomically marks a refresh token as consumed via
// SET NX — the key is the same revoked:refresh:<token> key IsTokenRevoked
// checks, so a consumed token reads as revoked to every VerifyRefreshToken
// path. Returns true only for the caller that won the consume; concurrent
// rotations of the same token can no longer both succeed.
//
// The marker value distinguishes rotation-consumption ("<unixTs>|<sessionID>")
// from plain revocation ("1", written by logout/password-change paths) so
// reuse detection can tell a rotated-away token from a revoked one.
func (r *RedisRevocationRepository) ConsumeRefreshToken(ctx context.Context, token string, marker string, ttl time.Duration) (bool, error) {
	if ttl < minRedisTTL {
		ttl = minRedisTTL
	}
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	ok, err := r.client.SetNX(ctx, revokedRefreshPrefix+token, marker, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("consume refresh token: %w", err)
	}
	return ok, nil
}

// UnconsumeRefreshToken removes a consumed marker iff it still holds the
// caller's marker — used to roll back a consume when storing the rotated
// pair fails, so a straggler can re-attempt the rotation instead of the
// session wedging in consumed-with-no-successor state.
func (r *RedisRevocationRepository) UnconsumeRefreshToken(ctx context.Context, token string, marker string) error {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return unconsumeScript.Run(ctx, r.client, []string{revokedRefreshPrefix + token}, marker).Err()
}

// StoreRotatedRefresh caches the refresh token issued for a consumed one so
// stragglers presenting the just-rotated token within the grace window get
// the same pair instead of tripping reuse detection.
func (r *RedisRevocationRepository) StoreRotatedRefresh(ctx context.Context, token string, newToken string, ttl time.Duration) error {
	if ttl < minRedisTTL {
		ttl = minRedisTTL
	}
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return r.client.Set(ctx, rotatedRefreshPrefix+token, newToken, ttl).Err()
}

// RotatedRefreshFor returns the replacement refresh token for a consumed one,
// if still within the grace window.
func (r *RedisRevocationRepository) RotatedRefreshFor(ctx context.Context, token string) (string, bool, error) {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	val, err := r.client.Get(ctx, rotatedRefreshPrefix+token).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get rotated refresh: %w", err)
	}
	return val, true, nil
}

// MarkUserRevoked stores a cutoff timestamp for a user: every token (access
// or refresh) issued at or before that instant is treated as revoked. This
// is how password reset/change kills all of a user's sessions without
// knowing the individual session IDs. The TTL should cover the longest-lived
// token (refresh expiry) so the marker cleans itself up.
func (r *RedisRevocationRepository) MarkUserRevoked(ctx context.Context, userID string, revokedAt time.Time, ttl time.Duration) error {
	if ttl < minRedisTTL {
		ttl = minRedisTTL
	}
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return r.client.Set(ctx, revokedUserPrefix+userID, strconv.FormatInt(revokedAt.Unix(), 10), ttl).Err()
}

// IsUserRevoked reports whether a token issued at `issuedAt` predates the
// user's revocation cutoff — i.e. the token was minted before the password
// reset/change that killed the sessions.
func (r *RedisRevocationRepository) IsUserRevoked(ctx context.Context, userID string, issuedAt time.Time) (bool, error) {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	val, err := r.client.Get(ctx, revokedUserPrefix+userID).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check user revocation: %w", err)
	}
	cutoff, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return false, fmt.Errorf("parse user revocation marker: %w", err)
	}
	// Tokens issued at or before the cutoff are dead. Comparison is at
	// unix-second granularity — biased toward revoking on ties, since a
	// token minted in the same second as a password reset can't be trusted.
	return issuedAt.Unix() <= cutoff, nil
}

// RefreshMarkerFor returns the marker stored on the revoked:refresh:<token>
// key ("" when absent). Lets the rotation path tell a recently-consumed
// token ("<unixTs>|<sessionID>") from one revoked by logout ("1").
func (r *RedisRevocationRepository) RefreshMarkerFor(ctx context.Context, token string) (string, bool, error) {
	ctx, cancel := redisutil.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	val, err := r.client.Get(ctx, revokedRefreshPrefix+token).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get refresh marker: %w", err)
	}
	return val, true, nil
}
