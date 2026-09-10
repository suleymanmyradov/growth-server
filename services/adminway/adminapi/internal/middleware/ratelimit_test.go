package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeromicro/go-zero/core/stores/redis"

	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
)

func TestClassifyAdminEndpoint(t *testing.T) {
	tests := []struct {
		path   string
		method string
		want   sharedmw.RateBucket
	}{
		{path: "/api/v1/admin/auth/login", method: http.MethodPost, want: sharedmw.RateBucketAuth},
		{path: "/api/v1/admin/auth/refresh", method: http.MethodPost, want: sharedmw.RateBucketAuth},
		{path: "/api/v1/admin/auth/unknown", method: http.MethodPost, want: sharedmw.RateBucketAuth},
		{path: "/api/v1/admin/articles", method: http.MethodGet, want: sharedmw.RateBucketNone},
		{path: "/api/v1/admin/articles", method: http.MethodPost, want: sharedmw.RateBucketNone},
		{path: "/api/v1/admin/site-settings", method: http.MethodPut, want: sharedmw.RateBucketNone},
		{path: "/api/v1/admin/metrics/activation", method: http.MethodGet, want: sharedmw.RateBucketNone},
		{path: "/health", method: http.MethodGet, want: sharedmw.RateBucketNone},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, classifyAdminEndpoint(tt.path, tt.method))
		})
	}
}

// The auth bucket must enforce the configured per-IP quota: after the quota is
// exhausted, further requests get 429 even though the route itself is public.
func TestRateLimitMiddleware_AuthQuotaEnforced(t *testing.T) {
	mr := miniredis.RunT(t)

	limiters := BuildRateLimiters(RateLimitConfig{
		Redis: redis.RedisConf{
			Host: mr.Addr(),
			Type: "node",
		},
		AuthQuota:   "60,3",
		AIQuota:     "60,10",
		SearchQuota: "60,30",
	})
	require.NotNil(t, limiters)

	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	wrapped := RateLimitMiddleware(limiters)(handler)

	do := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", nil)
		rec := httptest.NewRecorder()
		wrapped(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do())
	assert.Equal(t, http.StatusOK, do())
	assert.Equal(t, http.StatusOK, do())
	assert.Equal(t, http.StatusTooManyRequests, do(), "4th request within the window must be limited")
	assert.Equal(t, http.StatusTooManyRequests, do())

	// Non-auth routes are untouched by the auth bucket.
	other := httptest.NewRequest(http.MethodGet, "/api/v1/admin/articles", nil)
	otherRec := httptest.NewRecorder()
	wrapped(otherRec, other)
	assert.Equal(t, http.StatusOK, otherRec.Code)
}

// Without a Redis host the limiters are nil and the middleware is a no-op.
func TestBuildRateLimiters_DisabledWithoutRedis(t *testing.T) {
	assert.Nil(t, BuildRateLimiters(RateLimitConfig{}))
}

// The Namespace setting must isolate Redis keys per service so gateways sharing
// one Redis instance do not draw from each other's buckets.
func TestBuildRateLimiters_NamespacedKeys(t *testing.T) {
	mr := miniredis.RunT(t)

	limiters := BuildRateLimiters(RateLimitConfig{
		Redis:       redis.RedisConf{Host: mr.Addr(), Type: "node"},
		Namespace:   "adminway",
		AuthQuota:   "60,1",
		AIQuota:     "60,10",
		SearchQuota: "60,30",
	})
	require.NotNil(t, limiters)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", nil)
	wrapped := RateLimitMiddleware(limiters)(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	keys := mr.Keys()
	require.NotEmpty(t, keys)
	for _, key := range keys {
		assert.True(t, strings.HasPrefix(key, "adminway:auth"), "keys must be namespaced, got %q", key)
	}
}
