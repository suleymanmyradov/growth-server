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

// BuildRateLimiters creates PeriodLimit limiters from config. If Redis is not configured,
// it returns nil limiters (rate limiting is disabled).
func BuildRateLimiters(cfg RateLimitConfig) *RateLimiters {
	if cfg.Redis.Host == "" {
		logx.Info("rate limiting disabled: no Redis host configured")
		return nil
	}

	store := cfg.Redis.NewRedis()
	return &RateLimiters{
		AuthLimiter:   newPeriodLimit(cfg.AuthQuota, store, "ratelimit:auth"),
		AILimiter:     newPeriodLimit(cfg.AIQuota, store, "ratelimit:ai"),
		SearchLimiter: newPeriodLimit(cfg.SearchQuota, store, "ratelimit:search"),
	}
}

// RateLimitMiddleware returns a go-zero rest.Middleware that applies different
// rate limits based on the request path. Auth endpoints are limited per-IP,
// AI endpoints are limited per-authenticated-user, and public endpoints like
// search are limited per-IP.
func RateLimitMiddleware(limiters *RateLimiters) rest.Middleware {
	if limiters == nil {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return next
		}
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			method := r.Method

			switch {
			// Auth endpoints: per-IP brute-force protection
			case isAuthEndpoint(path, method):
				if !allowIP(limiters.AuthLimiter, r) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}

			// AI endpoints: per-user RPM limit
			case isAIEndpoint(path, method):
				if !allowUser(limiters.AILimiter, r) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}

			// Public search: per-IP limit
			case isSearchEndpoint(path, method):
				if !allowIP(limiters.SearchLimiter, r) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			}

			next(w, r)
		}
	}
}

func isAuthEndpoint(path, method string) bool {
	return path == "/api/v1/auth/login" ||
		path == "/api/v1/auth/register" ||
		path == "/api/v1/auth/refresh"
}

func isAIEndpoint(path, method string) bool {
	// Conversational AI
	if strings.HasPrefix(path, "/api/v1/conversations/") && strings.HasSuffix(path, "/messages") && method == http.MethodPost {
		return true
	}
	// Check-in AI feedback
	if path == "/api/v1/check-ins" && method == http.MethodPost {
		return true
	}
	// Weekly review generation
	if path == "/api/v1/weekly-reviews/generate" && method == http.MethodPost {
		return true
	}
	// Personalized coaching generation
	if path == "/api/v1/personalization/generate-coaching" && method == http.MethodPost {
		return true
	}
	return false
}

func isSearchEndpoint(path, method string) bool {
	return path == "/api/v1/search" && method == http.MethodGet
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
