package middleware

import (
	"net/http"
	"strings"

	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
)

// RateLimitConfig is re-exported from the shared package so the adminway config
// struct can embed it without changing its import path.
type RateLimitConfig = sharedmw.RateLimitConfig

// RateLimiters is re-exported from the shared package.
type RateLimiters = sharedmw.RateLimiters

// BuildRateLimiters delegates to the shared implementation.
func BuildRateLimiters(cfg RateLimitConfig) *RateLimiters {
	return sharedmw.BuildRateLimiters(cfg)
}

// RateLimitMiddleware applies the adminway's endpoint classification on top of
// the shared rate-limit middleware.
func RateLimitMiddleware(limiters *RateLimiters) func(http.HandlerFunc) http.HandlerFunc {
	return sharedmw.RateLimitMiddleware(limiters, classifyAdminEndpoint)
}

// classifyAdminEndpoint maps a request to its rate-limit bucket based on the
// adminway route table. Only the unauthenticated auth routes (login, refresh)
// get the per-IP auth bucket — every other adminway route already requires
// Auth+AdminAuth middlewares.
func classifyAdminEndpoint(path, method string) sharedmw.RateBucket {
	if strings.HasPrefix(path, "/api/v1/admin/auth/") {
		return sharedmw.RateBucketAuth
	}
	return sharedmw.RateBucketNone
}
