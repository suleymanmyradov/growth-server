package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
)

func TestClassifyGatewayEndpoint_AuthRoutes(t *testing.T) {
	// Every unauthenticated route in the auth group must land in the per-IP
	// auth bucket (brute-force protection).
	authPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/refresh",
		"/api/v1/auth/verify-email",
		"/api/v1/auth/resend-verification",
		"/api/v1/auth/google",
		"/api/v1/auth/apple",
		"/api/v1/auth/forgot-password",
		"/api/v1/auth/reset-password",
	}
	for _, path := range authPaths {
		assert.Equal(t, sharedmw.RateBucketAuth, classifyGatewayEndpoint(path, http.MethodPost), path)
	}
}

func TestClassifyGatewayEndpoint_OtherBuckets(t *testing.T) {
	assert.Equal(t, sharedmw.RateBucketAI, classifyGatewayEndpoint("/api/v1/check-ins", http.MethodPost))
	assert.Equal(t, sharedmw.RateBucketSearch, classifyGatewayEndpoint("/api/v1/search", http.MethodGet))

	// Non-POST check-ins and non-GET search are not classified.
	assert.Equal(t, sharedmw.RateBucketNone, classifyGatewayEndpoint("/api/v1/check-ins", http.MethodGet))
	assert.Equal(t, sharedmw.RateBucketNone, classifyGatewayEndpoint("/api/v1/search", http.MethodPost))

	// Regular authenticated CRUD routes are never limited.
	assert.Equal(t, sharedmw.RateBucketNone, classifyGatewayEndpoint("/api/v1/habits", http.MethodGet))
	assert.Equal(t, sharedmw.RateBucketNone, classifyGatewayEndpoint("/", http.MethodGet))
}
