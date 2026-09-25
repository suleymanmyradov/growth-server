package google

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

const testAudience = "874343812843-test.apps.googleusercontent.com"

// testJWKS spins up a mock Google JWKS endpoint serving the public key for the
// given kid, and returns the server URL.
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

// newTestVerifier returns a verifier whose JWKS endpoint points at the test
// server.
func newTestVerifier(jwksURL string, audiences ...string) *IDTokenVerifier {
	v := NewIDTokenVerifier(audiences, nil)
	v.jwksURL = jwksURL
	return v
}

// mintIDToken builds a signed Google-shaped ID token using the test key.
func mintIDToken(t *testing.T, privKey *rsa.PrivateKey, kid, audience string, claims map[string]any) string {
	t.Helper()
	now := time.Now()
	mapClaims := jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            audience,
		"sub":            "112233445566778899",
		"email":          "fixture@example.com",
		"email_verified": true,
		"name":           "Fixture User",
		"given_name":     "Fixture",
		"family_name":    "User",
		"picture":        "https://example.com/pic.jpg",
		"iat":            now.Unix(),
		"exp":            now.Add(10 * time.Minute).Unix(),
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
	const kid = "test-kid-1"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, nil)
	ui, err := v.VerifyIDToken(context.Background(), idToken)
	require.NoError(t, err)
	assert.Equal(t, "112233445566778899", ui.Subject)
	assert.Equal(t, "fixture@example.com", ui.Email)
	assert.True(t, ui.EmailVerified)
	assert.Equal(t, "Fixture User", ui.Name)
	assert.Equal(t, "Fixture", ui.GivenName)
	assert.Equal(t, "User", ui.FamilyName)
	assert.Equal(t, "https://example.com/pic.jpg", ui.Picture)
}

func TestVerifyIDToken_AcceptsLegacyIssuer(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-iss"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, map[string]any{
		"iss": "accounts.google.com",
	})
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.NoError(t, err)
}

func TestVerifyIDToken_AcceptsAnyConfiguredAudience(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-multi"
	jwksURL := testJWKS(t, kid, privKey)

	// Web + iOS + Android client IDs all allowed.
	v := newTestVerifier(jwksURL, "web-client-id", testAudience, "android-client-id")

	idToken := mintIDToken(t, privKey, kid, testAudience, nil)
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.NoError(t, err)
}

func TestVerifyIDToken_RejectsWrongAudience(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-2"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, "attacker-client-id.apps.googleusercontent.com", nil)
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audience")
}

func TestVerifyIDToken_RejectsWrongIssuer(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-3"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, map[string]any{
		"iss": "https://evil.example.com",
	})
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issuer")
}

func TestVerifyIDToken_RejectsExpiredToken(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-4"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, map[string]any{
		"exp": time.Now().Add(-1 * time.Minute).Unix(),
		"iat": time.Now().Add(-10 * time.Minute).Unix(),
	})
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.Error(t, err)
}

func TestVerifyIDToken_RejectsWrongSigningKey(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-5"
	// JWKS serves otherKey, but the token is signed with privKey.
	jwksURL := testJWKS(t, kid, otherKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, nil)
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.Error(t, err)
}

func TestVerifyIDToken_RejectsUnknownKID(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwksURL := testJWKS(t, "kid-in-jwks", privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, "kid-not-in-jwks", testAudience, nil)
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.Error(t, err)
}

func TestVerifyIDToken_RejectsMissingSub(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-6"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, map[string]any{
		"sub": "",
	})
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sub")
}

func TestVerifyIDToken_RejectsMissingEmail(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-7"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, map[string]any{
		"email": "",
	})
	_, err = v.VerifyIDToken(context.Background(), idToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}

func TestVerifyIDToken_EmailVerifiedAsString(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "test-kid-8"
	jwksURL := testJWKS(t, kid, privKey)

	v := newTestVerifier(jwksURL, testAudience)

	idToken := mintIDToken(t, privKey, kid, testAudience, map[string]any{
		"email_verified": "true",
	})
	ui, err := v.VerifyIDToken(context.Background(), idToken)
	require.NoError(t, err)
	assert.True(t, ui.EmailVerified)
}

func TestVerifyIDToken_RejectsEmptyToken(t *testing.T) {
	v := NewIDTokenVerifier([]string{testAudience}, nil)
	_, err := v.VerifyIDToken(context.Background(), "")
	require.Error(t, err)
}

func TestVerifyIDToken_RejectsNoAudiencesConfigured(t *testing.T) {
	v := NewIDTokenVerifier(nil, nil)
	_, err := v.VerifyIDToken(context.Background(), "any-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audiences")
}
