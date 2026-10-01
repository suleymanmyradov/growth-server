package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
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
	// TrustedProxies lists CIDR ranges or IPs of the reverse proxies that are
	// allowed to assert the client address via X-Forwarded-For / X-Real-Ip
	// (e.g. the Caddy container's Docker-network addresses). Requests whose
	// immediate peer is NOT in this set have those headers ignored, so clients
	// cannot spoof their IP to rotate rate-limit buckets.
	TrustedProxies []string `json:",optional"`
}

// RateLimiters holds initialized go-zero PeriodLimit limiters.
type RateLimiters struct {
	AuthLimiter    *limit.PeriodLimit
	AILimiter      *limit.PeriodLimit
	SearchLimiter  *limit.PeriodLimit
	trustedProxies *proxySet
}

// RateBucket identifies which rate-limit bucket a request falls into.
type RateBucket int

const (
	// RateBucketNone means no rate limiting applies to the request.
	RateBucketNone RateBucket = iota
	// RateBucketAuth is the per-account+IP bucket for auth endpoints.
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

// proxySet is the parsed form of TrustedProxies: a set of CIDR prefixes and
// exact IPs (stored as full-length prefixes).
type proxySet struct {
	prefixes []netip.Prefix
}

// newProxySet parses CIDR ranges and plain IPs into a proxySet. An empty input
// yields an empty set, which trusts no proxy headers at all.
func newProxySet(entries []string) (*proxySet, error) {
	ps := &proxySet{}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			p, err := netip.ParsePrefix(e)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", e, err)
			}
			ps.prefixes = append(ps.prefixes, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(e)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy IP %q: %w", e, err)
		}
		ps.prefixes = append(ps.prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return ps, nil
}

// contains reports whether ip falls inside any trusted proxy range.
func (ps *proxySet) contains(ip string) bool {
	if ps == nil {
		return false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range ps.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

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

	trusted, err := newProxySet(cfg.TrustedProxies)
	if err != nil {
		logx.Must(err)
	}

	store := cfg.Redis.NewRedis()
	return &RateLimiters{
		AuthLimiter:    newPeriodLimit(cfg.AuthQuota, store, namespace+":auth"),
		AILimiter:      newPeriodLimit(cfg.AIQuota, store, namespace+":ai"),
		SearchLimiter:  newPeriodLimit(cfg.SearchQuota, store, namespace+":search"),
		trustedProxies: trusted,
	}
}

// RateLimitMiddleware returns a go-zero rest.Middleware that applies the
// appropriate rate limit based on the ClassifyFunc's bucket assignment.
// The auth bucket is limited per account+IP (when an account identifier can be
// read from the JSON body — e.g. the login email — otherwise per IP), the
// search bucket per IP, and the AI bucket per authenticated user.
//
// verifier is used on AI-bucket requests to key the limiter by the bearer
// token's subject when this middleware runs before the route-level JWT
// middleware (global middleware executes first in go-zero, so the principal
// is not yet in context). Without it, every AI request — including
// authenticated ones — would silently share a per-IP bucket. Pass nil only
// for services whose classify func never returns RateBucketAI (e.g. adminway).
// If limiters is nil, rate limiting is disabled and the middleware is a no-op.
func RateLimitMiddleware(limiters *RateLimiters, classify ClassifyFunc, verifier AccessTokenVerifier) rest.Middleware {
	if limiters == nil {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return next
		}
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			switch classify(r.URL.Path, r.Method) {
			case RateBucketAuth:
				if !allowAuth(limiters.AuthLimiter, r, limiters.trustedProxies) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			case RateBucketAI:
				if !allowUser(limiters.AILimiter, r, limiters.trustedProxies, verifier) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			case RateBucketSearch:
				if !allowIP(limiters.SearchLimiter, r, limiters.trustedProxies) {
					errors.WriteError(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			}

			next(w, r)
		}
	}
}

func take(limiter *limit.PeriodLimit, r *http.Request, key, what string) bool {
	code, err := limiter.TakeCtx(r.Context(), key)
	if err != nil {
		logx.WithContext(r.Context()).Errorf("rate limit error for %s %s: %v", what, key, err)
		// Fail closed: if Redis is unreachable, block the request
		return false
	}
	return code == limit.Allowed || code == limit.HitQuota
}

// allowAuth limits auth endpoints by account+IP: all login traffic from one
// shared IP (NAT, carrier-grade NAT, an office) no longer drains a single
// bucket and locks out every account, while brute force against one account
// from one IP stays bounded. Falls back to IP-only when no account identifier
// is present in the body (e.g. token refresh).
func allowAuth(limiter *limit.PeriodLimit, r *http.Request, trusted *proxySet) bool {
	ip := realIP(r, trusted)
	key := ip
	if acct := extractAccountID(r); acct != "" {
		// Hash the identifier so account emails never sit in Redis keys or
		// rate-limit error logs in plaintext.
		sum := sha256.Sum256([]byte(acct))
		key = ip + "|" + hex.EncodeToString(sum[:])[:24]
	}
	return take(limiter, r, key, "auth")
}

func allowIP(limiter *limit.PeriodLimit, r *http.Request, trusted *proxySet) bool {
	return take(limiter, r, realIP(r, trusted), "IP")
}

func allowUser(limiter *limit.PeriodLimit, r *http.Request, trusted *proxySet, verifier AccessTokenVerifier) bool {
	p, ok := principal.PrincipalFrom(r.Context())
	if ok && p.UserID != "" {
		return take(limiter, r, p.UserID, "user")
	}
	// The principal is absent when this middleware runs before route-level
	// auth (global middleware executes first). Verify the bearer token
	// directly so authenticated AI traffic is keyed by user, not IP —
	// otherwise every user behind a shared IP/NAT drains one bucket, and an
	// attacker rotating IPs gets an effectively unbounded budget.
	if verifier != nil {
		if tok := bearerToken(r); tok != "" {
			if claims, err := verifier.VerifyAccessToken(r.Context(), tok); err == nil {
				return take(limiter, r, claims.Subject.String(), "user")
			}
		}
	}
	// No valid credentials: fall back to IP to prevent anonymous abuse.
	return allowIP(limiter, r, trusted)
}

// bearerToken extracts the token from "Authorization: Bearer <token>",
// returning "" when absent or malformed. It performs no validation —
// verification is the verifier's job.
func bearerToken(r *http.Request) string {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

// accountFields are the JSON body keys inspected for a login identifier on
// auth endpoints. Covers the gateway (email) and adminway (email) login forms
// plus plausible aliases.
var accountFields = []string{"email", "username", "identifier", "login"}

// extractAccountID peeks at a JSON request body for an account identifier
// (email/username) without consuming it — the body is fully restored for the
// downstream handler. Returns a normalized, length-bounded identifier or "".
func extractAccountID(r *http.Request) string {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return ""
	}
	const maxBody = 64 << 10 // 64KB — auth bodies are a few hundred bytes.
	// Only peek at small, fully-buffered bodies. Oversized or chunked bodies
	// fall back to the IP-only bucket rather than being read partially.
	if r.ContentLength <= 0 || r.ContentLength > maxBody {
		return ""
	}
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "json") {
		return ""
	}

	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil || len(body) == 0 {
		return ""
	}

	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		return ""
	}
	for _, f := range accountFields {
		var s string
		if v, ok := raw[f]; ok && json.Unmarshal(v, &s) == nil {
			if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
				if len(s) > 320 {
					s = s[:320]
				}
				return s
			}
		}
	}
	return ""
}

// realIP extracts the client IP through the trusted proxy chain. When the
// immediate peer (RemoteAddr) is not a trusted proxy, X-Forwarded-For and
// X-Real-Ip are ignored entirely — clients cannot inject a fake client IP.
// When the peer IS trusted, the rightmost untrusted X-Forwarded-For entry is
// the client (the client cannot forge entries to the right of what trusted
// proxies append).
func realIP(r *http.Request, trusted *proxySet) string {
	remoteIP := hostOnly(r.RemoteAddr)
	if trusted == nil || !trusted.contains(remoteIP) {
		return remoteIP
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip == "" {
				continue
			}
			if !trusted.contains(ip) {
				return ip
			}
		}
		// Every hop is a trusted proxy: the leftmost entry is the client as
		// asserted by the outermost trusted proxy.
		return strings.TrimSpace(parts[0])
	}

	if xri := strings.TrimSpace(r.Header.Get("X-Real-Ip")); xri != "" {
		return xri
	}

	return remoteIP
}

// hostOnly strips the port from a "host:port" peer address.
func hostOnly(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
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
