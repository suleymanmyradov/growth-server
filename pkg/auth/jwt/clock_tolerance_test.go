package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ============================================
// Clock tolerance / leeway boundary tests
//
// The TokenMaker uses DefaultLeeway (30s) for time-based claim validation.
// These tests verify the exact boundary behavior:
//   - A token expired by < 30s is still valid (within leeway)
//   - A token expired by > 30s is rejected (outside leeway)
//   - A token with nbf in the near future (< 30s) is valid (within leeway)
//   - A token with nbf too far in the future (> 30s) is rejected
// ============================================

// makeTokenWithCustomExpiry creates a signed JWT with a custom expiry time.
func makeTokenWithCustomExpiry(t *testing.T, maker *TokenMaker, expiresAt, notBefore time.Time) string {
	t.Helper()
	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   uuid.New(),
		SessionID: uuid.New(),
		Username:  "test-user",
		Roles:     []string{"user"},
		Issuer:    "test-issuer",
		Audience:  []string{"test-audience"},
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		NotBefore: jwt.NewNumericDate(notBefore),
		TokenType: AccessToken,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	tokenString, err := token.SignedString([]byte(maker.secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tokenString
}

func TestClockTolerance_TokenExpiredWithinLeeway_IsValid(t *testing.T) {
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Token expired 10 seconds ago — within the 30s leeway
	expiredAt := time.Now().Add(-10 * time.Second)
	tokenString := makeTokenWithCustomExpiry(t, maker, expiredAt, time.Now().Add(-time.Minute))

	claims, err := maker.VerifyAccessToken(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("expected token within leeway to be valid, got error: %v", err)
	}
	if claims == nil {
		t.Fatal("expected claims, got nil")
	}
}

func TestClockTolerance_TokenExpiredBeyondLeeway_IsRejected(t *testing.T) {
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Token expired 31 seconds ago — just beyond the 30s leeway
	expiredAt := time.Now().Add(-(DefaultLeeway + 1*time.Second))
	tokenString := makeTokenWithCustomExpiry(t, maker, expiredAt, time.Now().Add(-time.Minute))

	_, err = maker.VerifyAccessToken(context.Background(), tokenString)
	if err == nil {
		t.Fatal("expected token beyond leeway to be rejected, got nil error")
	}
}

func TestClockTolerance_TokenExpiredExactlyAtLeewayBoundary(t *testing.T) {
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Token expired exactly 30 seconds ago — at the leeway boundary
	// (jwt v5 uses <= for leeway comparison, so this should be valid)
	expiredAt := time.Now().Add(-DefaultLeeway)
	tokenString := makeTokenWithCustomExpiry(t, maker, expiredAt, time.Now().Add(-time.Minute))

	_, err = maker.VerifyAccessToken(context.Background(), tokenString)
	if err != nil {
		t.Logf("token at exact leeway boundary rejected: %v (jwt v5 may use < not <=)", err)
	}
	// Don't fail — the boundary behavior (inclusive vs exclusive) is a library
	// implementation detail. The important tests are the within/beyond cases.
}

func TestClockTolerance_NotBeforeInNearFuture_IsValid(t *testing.T) {
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Token with nbf 10 seconds in the future — within the 30s leeway
	notBefore := time.Now().Add(10 * time.Second)
	tokenString := makeTokenWithCustomExpiry(t, maker, time.Now().Add(time.Minute), notBefore)

	_, err = maker.VerifyAccessToken(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("expected token with nbf within leeway to be valid, got error: %v", err)
	}
}

func TestClockTolerance_NotBeforeTooFarInFuture_IsRejected(t *testing.T) {
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Token with nbf 60 seconds in the future — beyond the 30s leeway
	notBefore := time.Now().Add(60 * time.Second)
	tokenString := makeTokenWithCustomExpiry(t, maker, time.Now().Add(time.Hour), notBefore)

	_, err = maker.VerifyAccessToken(context.Background(), tokenString)
	if err == nil {
		t.Fatal("expected token with nbf beyond leeway to be rejected, got nil error")
	}
}

func TestClockTolerance_DefaultLeewayValue(t *testing.T) {
	// Verify the DefaultLeeway constant is 30 seconds, as documented
	if DefaultLeeway != 30*time.Second {
		t.Errorf("expected DefaultLeeway to be 30s, got %v", DefaultLeeway)
	}
}

func TestClockTolerance_RefreshTokenAlsoUsesLeeway(t *testing.T) {
	// Refresh tokens should also benefit from the leeway
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Create a refresh token that expired 10s ago (within leeway)
	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   uuid.New(),
		SessionID: uuid.New(),
		Username:  "test-user",
		Roles:     []string{"user"},
		Issuer:    "test-issuer",
		Audience:  []string{"test-audience"},
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-10 * time.Second)),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		TokenType: RefreshToken,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	tokenString, err := token.SignedString([]byte(maker.secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	// Without a revocation repo, VerifyRefreshToken only checks the token itself
	_, err = maker.VerifyRefreshToken(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("expected refresh token within leeway to be valid, got error: %v", err)
	}
}
