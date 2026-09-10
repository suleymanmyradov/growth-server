package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/zeromicro/go-zero/core/limit"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

// RateLimitConfig holds per-endpoint rate limit settings backed by Redis.
type RateLimitConfig struct {
	Redis redis.RedisConf
	// Namespace prefixes the Redis keys so services sharing one Redis instance
	// do not draw from each other's buckets (e.g. "gateway", "adminway").
	// Defaults to "ratelimit" when empty.
	Namespace string `json:",optional"`
	// AuthQuota is the per-IP fixed-window quota for auth endpoints (login/register/refresh).
	// Format: periodSeconds,quota (e.g., "60,5" means 5 requests per 60 seconds).
	AuthQuota string `json:",default=60,5"`
	// AIQuota is the per-user fixed-window quota for AI endpoints (RPM).
	// Format: periodSeconds,quota (e.g., "60,10" means 10 requests per 60 seconds).
	AIQuota string `json:",default=60,10"`
	// SearchQuota is the per-IP fixed-window quota for public search.
	// Format: periodSeconds,quota (e.g., "60,30" means 30 requests per 60 seconds).
	SearchQuota string `json:",default=60,30"`
}

// RateLimiters holds initialized go-zero PeriodLimit limiters.
type RateLimiters struct {
	AuthLimiter   *limit.PeriodLimit
	AILimiter     *limit.PeriodLimit
	SearchLimiter *limit.PeriodLimit
}

// RateBucket identifies which rate-limit bucket a request falls into.
type RateBucket int

const (
	// RateBucketNone means no rate limiting applies to the request.
	RateBucketNone RateBucket = iota
	// RateBucketAuth is the per-IP bucket for auth endpoints.
	RateBucketAuth
	// RateBucketAI is the per-user bucket for AI endpoints.
	RateBucketAI
	// RateBucketSearch is the per-IP bucket for public search.
	RateBucketSearch
)

// ClassifyFunc classifies a request into a rate-limit bucket based on its
// path and method. Each API service provides its own classifier that knows
// which endpoints belong to which bucket.
type ClassifyFunc func(path, method string) RateBucket

// BuildRateLimiters creates PeriodLimit limiters from config. If Redis is not configured,
// it returns nil limiters (rate limiting is disabled).
func BuildRateLimiters(cfg RateLimitConfig) *RateLimiters {
	if cfg.Redis.Host == "" {
		logx.Info("rate limiting disabled: no Redis host configured")
		return nil
	}

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "ratelimit"
	}

	store := cfg.Redis.NewRedis()
	return &RateLimiters{
		AuthLimiter:   newPeriodLimit(cfg.AuthQuota, store, namespace+":auth"),
		AILimiter:     newPeriodLimit(cfg.AIQuota, store, namespace+":ai"),
		SearchLimiter: newPeriodLimit(cfg.SearchQuota, store, namespace+":search"),
	}
}

// RateLimitMiddleware returns a go-zero rest.Middleware that applies the
// appropriate rate limit based on the ClassifyFunc's bucket assignment.
// Auth and search buckets are limited per-IP; AI buckets are limited
// per-authenticated-user (falling back to IP for anonymous requests).
// If limiters is nil, rate limiting is disabled and the middleware is a no-op.
func RateLimitMiddleware(limiters *RateLimiters, classify ClassifyFunc) rest.Middleware {
	if limiters == nil {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return next
		}
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			switch classify(r.URL.Path, r.Method) {
			case RateBucketAuth:
				if !allowIP(limiters.AuthLimiter, r) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			case RateBucketAI:
				if !allowUser(limiters.AILimiter, r) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			case RateBucketSearch:
				if !allowIP(limiters.SearchLimiter, r) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			}

			next(w, r)
		}
	}
}

func allowIP(limiter *limit.PeriodLimit, r *http.Request) bool {
	ip := realIP(r)
	code, err := limiter.TakeCtx(r.Context(), ip)
	if err != nil {
		logx.WithContext(r.Context()).Errorf("rate limit error for IP %s: %v", ip, err)
		// Fail closed: if Redis is unreachable, block the request
		return false
	}
	return code == limit.Allowed || code == limit.HitQuota
}

func allowUser(limiter *limit.PeriodLimit, r *http.Request) bool {
	p, ok := principal.PrincipalFrom(r.Context())
	if !ok || p.UserID == "" {
		// No authenticated user: fall back to IP to prevent anonymous abuse
		return allowIP(limiter, r)
	}
	code, err := limiter.TakeCtx(r.Context(), p.UserID)
	if err != nil {
		logx.WithContext(r.Context()).Errorf("rate limit error for user %s: %v", p.UserID, err)
		return false
	}
	return code == limit.Allowed || code == limit.HitQuota
}

// realIP extracts the client IP, preferring X-Forwarded-For / X-Real-Ip
// but falling back to RemoteAddr. Only the leftmost (closest to client) IP
// is used to prevent spoofing via the rightmost proxy IPs.
func realIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For: client, proxy1, proxy2
		// We take the first (leftmost) as the real client IP.
		if idx := strings.Index(xff, ","); idx != -1 {
			xff = strings.TrimSpace(xff[:idx])
		}
		if xff != "" {
			return xff
		}
	}

	xri := r.Header.Get("X-Real-Ip")
	if xri != "" {
		return xri
	}

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// newPeriodLimit parses a "period,quota" string and creates a PeriodLimit.
func newPeriodLimit(periodQuota string, store *redis.Redis, keyPrefix string) *limit.PeriodLimit {
	parts := strings.Split(periodQuota, ",")
	if len(parts) != 2 {
		logx.Must(fmt.Errorf("invalid rate limit format %q, expected period,quota", periodQuota))
	}
	period := 0
	quota := 0
	if _, err := fmt.Sscanf(parts[0], "%d", &period); err != nil {
		logx.Must(fmt.Errorf("invalid rate limit period %q: %v", parts[0], err))
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &quota); err != nil {
		logx.Must(fmt.Errorf("invalid rate limit quota %q: %v", parts[1], err))
	}
	return limit.NewPeriodLimit(period, quota, store, keyPrefix)
}
