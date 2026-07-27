package apple

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testJWKS spins up a mock Apple JWKS endpoint serving the public key for the
// given kid, and returns the server URL and the kid.
func testJWKS(t *testing.T, kid string, privKey *rsa.PrivateKey) string {
	t.Helper()
	pub := &privKey.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	eBytes := []byte{byte(pub.E >> 24), byte(pub.E >> 16), byte(pub.E >> 8), byte(pub.E)}
	// Trim leading zero bytes for canonical encoding.
	eStr := base64.RawURLEncoding.EncodeToString(eBytes)
	jwks := map[string]any{
		"keys": []map[string]any{
			{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": n, "e": eStr},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// mintAppleIDToken builds a signed Apple-shaped ID token using the test key.
func mintAppleIDToken(t *testing.T, privKey *rsa.PrivateKey, kid, serviceID string, claims map[string]any) string {
	t.Helper()
	now := time.Now()
	mapClaims := jwt.MapClaims{
		"iss":             appleIssuer,
		"aud":             serviceID,
		"sub":             "001234.abc.def.ghi",
		"email":           "fixture@example.com",
		"email_verified":  "true",
		"is_private_email": "false",
		"iat":             now.Unix(),
		"exp":             now.Add(10 * time.Minute).Unix(),
	}
	for k, v := range claims {
		mapClaims[k] = v
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, mapClaims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(privKey)
	require.NoError(t, err)
	return signed
}

func TestVerifyIDToken_HappyPath(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-1", "com.example.growth"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, privKey, kid, serviceID, nil)
	ui, err := v.VerifyIDToken(context.Background(), idToken, "")
	require.NoError(t, err)
	assert.Equal(t, "001234.abc.def.ghi", ui.Subject)
	assert.Equal(t, "fixture@example.com", ui.Email)
	assert.True(t, ui.EmailVerified)
	assert.False(t, ui.IsPrivateRelayEmail)
}

func TestVerifyIDToken_PrivateRelayEmail(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-2", "com.example.growth"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, privKey, kid, serviceID, map[string]any{
		"email":            "abc@privaterelay.appleid.com",
		"is_private_email": "true",
	})
	ui, err := v.VerifyIDToken(context.Background(), idToken, "")
	require.NoError(t, err)
	assert.True(t, ui.IsPrivateRelayEmail)
	assert.Equal(t, "abc@privaterelay.appleid.com", ui.Email)
}

func TestVerifyIDToken_RejectsWrongAudience(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-3"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: "com.example.growth"}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, privKey, kid, "com.different.service", nil)
	_, err = v.VerifyIDToken(context.Background(), idToken, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify id token")
}

func TestVerifyIDToken_RejectsExpiredToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-4", "com.example.growth"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, privKey, kid, serviceID, map[string]any{
		"exp": time.Now().Add(-1 * time.Minute).Unix(),
		"iat": time.Now().Add(-10 * time.Minute).Unix(),
	})
	_, err = v.VerifyIDToken(context.Background(), idToken, "")
	require.Error(t, err)
}

func TestVerifyIDToken_RejectsWrongIssuer(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-5", "com.example.growth"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, privKey, kid, serviceID, map[string]any{
		"iss": "https://evil.example.com",
	})
	_, err = v.VerifyIDToken(context.Background(), idToken, "")
	require.Error(t, err)
}

func TestVerifyIDToken_RejectsBadSignature(t *testing.T) {
	signerKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwksKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-6", "com.example.growth"
	// JWKS serves a DIFFERENT key than the one that signed the token.
	jwksURL := testJWKS(t, kid, jwksKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, signerKey, kid, serviceID, nil)
	_, err = v.VerifyIDToken(context.Background(), idToken, "")
	require.Error(t, err)
}

func TestVerifyIDToken_NonceMatch(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-7", "com.example.growth"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, privKey, kid, serviceID, map[string]any{
		"nonce": "abc123",
	})
	// Matching nonce succeeds.
	_, err = v.VerifyIDToken(context.Background(), idToken, "abc123")
	require.NoError(t, err)
	// Mismatched nonce fails.
	_, err = v.VerifyIDToken(context.Background(), idToken, "wrong")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonce mismatch")
}

func TestVerifyIDToken_MissingEmailRejected(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-8", "com.example.growth"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	idToken := mintAppleIDToken(t, privKey, kid, serviceID, map[string]any{
		"email": "",
	})
	_, err = v.VerifyIDToken(context.Background(), idToken, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing email")
}

func TestVerifyIDToken_CachesJWKS(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-9", "com.example.growth"

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		pub := &privKey.PublicKey
		n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
		eBytes := []byte{byte(pub.E >> 24), byte(pub.E >> 16), byte(pub.E >> 8), byte(pub.E)}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{"kty": "RSA", "kid": kid, "alg": "RS256", "n": n, "e": base64.RawURLEncoding.EncodeToString(eBytes)},
			},
		})
	}))
	t.Cleanup(srv.Close)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = srv.URL

	idToken := mintAppleIDToken(t, privKey, kid, serviceID, nil)
	for i := 0; i < 3; i++ {
		_, err := v.VerifyIDToken(context.Background(), idToken, "")
		require.NoError(t, err)
	}
	// JWKS fetched exactly once due to in-memory cache.
	assert.Equal(t, 1, hits)
}

func TestFullName(t *testing.T) {
	assert.Equal(t, "John Doe", FullName("John", "Doe"))
	assert.Equal(t, "John", FullName("John", ""))
	assert.Equal(t, "Doe", FullName("", "Doe"))
	assert.Equal(t, "", FullName("", ""))
	assert.Equal(t, "John Doe", FullName("  John  ", "  Doe  "))
}

func TestIsAllowedRedirectURI(t *testing.T) {
	allowlist := []string{"https://app.example.com/auth/callback/apple", "growth://auth/apple"}
	assert.True(t, IsAllowedRedirectURI("growth://auth/apple", allowlist, "https://default.example.com"))
	assert.True(t, IsAllowedRedirectURI("https://default.example.com", nil, "https://default.example.com"))
	assert.False(t, IsAllowedRedirectURI("https://evil.example.com", allowlist, "https://default.example.com"))
	assert.False(t, IsAllowedRedirectURI("", allowlist, "https://default.example.com"))
}

func TestVerifyIDToken_EmptyInputs(t *testing.T) {
	v := NewVerifier(Config{ServiceID: "com.example.growth"}, nil)
	_, err := v.VerifyIDToken(context.Background(), "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id token is required")

	// Missing ServiceID.
	v2 := NewVerifier(Config{}, nil)
	_, err = v2.VerifyIDToken(context.Background(), "some.token.here", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ServiceID is not configured")
}

func TestVerifyIDToken_RejectsNonRS256Alg(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid, serviceID = "test-kid-10", "com.example.growth"
	jwksURL := testJWKS(t, kid, privKey)

	v := NewVerifier(Config{ServiceID: serviceID}, nil)
	v.jwksURL = jwksURL

	// Build a token signed with HS256 (symmetric) — must be rejected.
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": appleIssuer, "aud": serviceID, "sub": "x",
		"email": "fixture@example.com", "email_verified": "true",
		"iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(),
	})
	token.Header["kid"] = kid
	signed, err := token.SignedString([]byte("shared-secret"))
	require.NoError(t, err)
	_, err = v.VerifyIDToken(context.Background(), signed, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signing method")
}
