package middleware

import (
	"net/http"
	"strings"

	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
)

// RateLimitConfig is re-exported from the shared package so the gateway config
// struct can embed it without changing its import path.
type RateLimitConfig = sharedmw.RateLimitConfig

// RateLimiters is re-exported from the shared package.
type RateLimiters = sharedmw.RateLimiters

// BuildRateLimiters delegates to the shared implementation.
func BuildRateLimiters(cfg RateLimitConfig) *RateLimiters {
	return sharedmw.BuildRateLimiters(cfg)
}

// RateLimitMiddleware applies the gateway's endpoint classification on top of
// the shared rate-limit middleware.
func RateLimitMiddleware(limiters *RateLimiters) func(http.HandlerFunc) http.HandlerFunc {
	return sharedmw.RateLimitMiddleware(limiters, classifyGatewayEndpoint)
}

// classifyGatewayEndpoint maps a request to its rate-limit bucket based on the
// gateway's route table. After the AI/streaming routes moved to ai-gateway,
// the only remaining AI endpoint here is check-in creation (AI feedback).
func classifyGatewayEndpoint(path, method string) sharedmw.RateBucket {
	// Auth endpoints: per-IP brute-force protection. Covers every
	// unauthenticated route in the auth group (login, register, refresh,
	// verify-email, resend-verification, google/apple exchange, forgot/reset
	// password) — all of them are anonymous and abuse-prone.
	if strings.HasPrefix(path, "/api/v1/auth/") {
		return sharedmw.RateBucketAuth
	}

	// Check-in AI feedback: per-user RPM limit
	if path == "/api/v1/check-ins" && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}

	// Public search: per-IP limit
	if path == "/api/v1/search" && method == http.MethodGet {
		return sharedmw.RateBucketSearch
	}

	return sharedmw.RateBucketNone
}

// gatewayExemptPaths are endpoints whose responses intentionally do not use
// the standard data envelope (e.g. SSE endpoints still served by the gateway).
var gatewayExemptPaths = map[string]bool{
	"/api/v1/auth/register":   true,
	"/api/v1/auth/login":      true,
	"/api/v1/auth/refresh":    true,
	"/api/v1/auth/logout":     true,
	"/api/v1/billing/webhook": true,
}

// ResponseShapeMiddleware delegates to the shared implementation with the
// gateway's exempt paths.
func ResponseShapeMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return sharedmw.ResponseShapeMiddleware(gatewayExemptPaths)
}
