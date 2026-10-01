package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func mustProxySet(t *testing.T, entries ...string) *proxySet {
	t.Helper()
	ps, err := newProxySet(entries)
	if err != nil {
		t.Fatalf("newProxySet: %v", err)
	}
	return ps
}

func TestRealIP_UntrustedPeerIgnoresForwardedHeaders(t *testing.T) {
	// A client connecting directly (RemoteAddr is public, not a proxy) must
	// not be able to spoof its IP through forwarded headers.
	trusted := mustProxySet(t, "10.0.0.0/8")
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "203.0.113.9:4433"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-Ip", "5.6.7.8")
	if got := realIP(req, trusted); got != "203.0.113.9" {
		t.Fatalf("expected RemoteAddr to win for untrusted peer, got %q", got)
	}
}

func TestRealIP_TrustedProxyWalksChainRightToLeft(t *testing.T) {
	// Caddy (10.0.0.5) appends the inbound client IP to whatever the client
	// sent; the rightmost untrusted entry is the real client. The leftmost
	// entry here is client-controlled and must NOT be trusted.
	trusted := mustProxySet(t, "10.0.0.0/8")
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.5:80"
	req.Header.Set("X-Forwarded-For", "9.9.9.9, 198.51.100.7, 10.0.0.9")
	if got := realIP(req, trusted); got != "198.51.100.7" {
		t.Fatalf("expected rightmost untrusted XFF entry, got %q", got)
	}
}

func TestRealIP_AllTrustedFallsBackToLeftmost(t *testing.T) {
	trusted := mustProxySet(t, "10.0.0.0/8")
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.5:80"
	req.Header.Set("X-Forwarded-For", "10.0.0.7, 10.0.0.9")
	if got := realIP(req, trusted); got != "10.0.0.7" {
		t.Fatalf("expected leftmost entry for all-trusted chain, got %q", got)
	}
}

func TestRealIP_XRealIPOnlyFromTrustedPeer(t *testing.T) {
	trusted := mustProxySet(t, "10.0.0.0/8")
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.5:80"
	req.Header.Set("X-Real-Ip", "198.51.100.22")
	if got := realIP(req, trusted); got != "198.51.100.22" {
		t.Fatalf("expected X-Real-Ip from trusted peer, got %q", got)
	}
}

func TestRealIP_NilTrustSetNeverTrustsForwarded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.5:80"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := realIP(req, nil); got != "10.0.0.5" {
		t.Fatalf("expected RemoteAddr with nil trust set, got %q", got)
	}
}

func TestExtractAccountID(t *testing.T) {
	t.Run("email body", func(t *testing.T) {
		body := `{"email":"  Jane@Example.COM ","password":"x"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if got := extractAccountID(req); got != "jane@example.com" {
			t.Fatalf("expected normalized email, got %q", got)
		}
		rest, _ := io.ReadAll(req.Body)
		if string(rest) != body {
			t.Fatal("request body must be fully restored for the handler")
		}
	})

	t.Run("username field", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"username":"bob"}`))
		req.Header.Set("Content-Type", "application/json")
		if got := extractAccountID(req); got != "bob" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("no json content type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"email":"a@b.c"}`))
		req.Header.Set("Content-Type", "text/plain")
		if got := extractAccountID(req); got != "" {
			t.Fatalf("expected empty for non-json, got %q", got)
		}
	})

	t.Run("missing content type still parsed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"email":"a@b.c"}`))
		req.Header.Del("Content-Type")
		if got := extractAccountID(req); got != "a@b.c" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`not json`))
		if got := extractAccountID(req); got != "" {
			t.Fatalf("expected empty for invalid json, got %q", got)
		}
	})

	t.Run("oversized body skipped", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"email":"a@b.c"}`))
		req.ContentLength = 128 << 10
		if got := extractAccountID(req); got != "" {
			t.Fatalf("expected empty for oversized body, got %q", got)
		}
	})

	t.Run("no account field", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"refreshToken":"abc"}`))
		if got := extractAccountID(req); got != "" {
			t.Fatalf("expected empty without account field, got %q", got)
		}
	})
}

// fakeVerifier implements AccessTokenVerifier for pre-auth rate-limit tests.
type fakeVerifier struct {
	claims *jwt.TokenClaims
	err    error
}

func (f fakeVerifier) VerifyAccessToken(_ context.Context, _ string) (*jwt.TokenClaims, error) {
	return f.claims, f.err
}

func aiRequest(t *testing.T, remoteAddr, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/personalization/coaching", nil)
	req.RemoteAddr = remoteAddr
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// newAILimitMiddleware builds the middleware with a 1-request AI bucket
// backed by miniredis, so a second hit is observable.
func newAILimitMiddleware(t *testing.T, verifier AccessTokenVerifier) (rest_handler http.HandlerFunc, cleanup func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	limiters := BuildRateLimiters(RateLimitConfig{
		Redis:       redis.RedisConf{Host: mr.Addr(), Type: "node"},
		AuthQuota:   "60,5",
		AIQuota:     "60,1",
		SearchQuota: "60,30",
	})
	if limiters == nil {
		t.Fatal("limiters nil — miniredis config failed")
	}
	mw := RateLimitMiddleware(limiters, func(path, method string) RateBucket { return RateBucketAI }, verifier)
	return mw(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }), func() { mr.Close() }
}

func statusOf(h http.HandlerFunc, r *http.Request) int {
	w := httptest.NewRecorder()
	h(w, r)
	return w.Code
}

func TestAllowUser_PerUserBucketViaPreAuthVerifier(t *testing.T) {
	userA := uuid.New()
	claims := &jwt.TokenClaims{Subject: userA}
	handler, cleanup := newAILimitMiddleware(t, fakeVerifier{claims: claims})
	defer cleanup()

	// First request from user A passes.
	if got := statusOf(handler, aiRequest(t, "203.0.113.1:1000", "tok")); got != http.StatusOK {
		t.Fatalf("first request = %d, want 200", got)
	}
	// Second request from the SAME user but a DIFFERENT IP is still the same
	// bucket — the limiter must be keyed by the verified token subject, not
	// the (rotatable) IP.
	if got := statusOf(handler, aiRequest(t, "198.51.100.7:2000", "tok")); got != http.StatusTooManyRequests {
		t.Fatalf("same user different IP = %d, want 429 (per-user bucket)", got)
	}
}

func TestAllowUser_DifferentUsersDontShareBucket(t *testing.T) {
	// Verifier maps token string to different subjects.
	userA, userB := uuid.New(), uuid.New()
	v := verifierFunc(func(_ context.Context, token string) (*jwt.TokenClaims, error) {
		if token == "tokB" {
			return &jwt.TokenClaims{Subject: userB}, nil
		}
		return &jwt.TokenClaims{Subject: userA}, nil
	})
	handler, cleanup := newAILimitMiddleware(t, v)
	defer cleanup()

	if got := statusOf(handler, aiRequest(t, "203.0.113.1:1000", "tokA")); got != http.StatusOK {
		t.Fatalf("user A first = %d, want 200", got)
	}
	// A different user on the SAME IP gets their own bucket.
	if got := statusOf(handler, aiRequest(t, "203.0.113.1:1000", "tokB")); got != http.StatusOK {
		t.Fatalf("user B same IP = %d, want 200 (own bucket)", got)
	}
}

type verifierFunc func(ctx context.Context, token string) (*jwt.TokenClaims, error)

func (f verifierFunc) VerifyAccessToken(ctx context.Context, token string) (*jwt.TokenClaims, error) {
	return f(ctx, token)
}

func TestAllowUser_InvalidTokenFallsBackToIP(t *testing.T) {
	v := fakeVerifier{err: errors.New("bad signature")}
	handler, cleanup := newAILimitMiddleware(t, v)
	defer cleanup()

	if got := statusOf(handler, aiRequest(t, "203.0.113.1:1000", "bad")); got != http.StatusOK {
		t.Fatalf("first = %d, want 200", got)
	}
	// Invalid token → IP bucket → second request same IP is blocked.
	if got := statusOf(handler, aiRequest(t, "203.0.113.1:1000", "bad2")); got != http.StatusTooManyRequests {
		t.Fatalf("invalid tokens same IP = %d, want 429 (IP fallback)", got)
	}
}

func TestAllowUser_NoTokenUsesIP(t *testing.T) {
	handler, cleanup := newAILimitMiddleware(t, fakeVerifier{claims: &jwt.TokenClaims{Subject: uuid.New()}})
	defer cleanup()

	if got := statusOf(handler, aiRequest(t, "203.0.113.1:1000", "")); got != http.StatusOK {
		t.Fatalf("first = %d, want 200", got)
	}
	if got := statusOf(handler, aiRequest(t, "203.0.113.1:1000", "")); got != http.StatusTooManyRequests {
		t.Fatalf("anonymous second = %d, want 429 (IP fallback)", got)
	}
}
