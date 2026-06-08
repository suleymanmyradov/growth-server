package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// mockRevocationRepo is a simple in-memory RevocationRepository for testing.
type mockRevocationRepo struct {
	revoked map[string]struct{}
}

func newMockRevocationRepo() *mockRevocationRepo {
	return &mockRevocationRepo{revoked: make(map[string]struct{})}
}

func (r *mockRevocationRepo) MarkTokenRevoke(_ context.Context, _ TokenType, token string, _ time.Duration) error {
	r.revoked[token] = struct{}{}
	return nil
}

func (r *mockRevocationRepo) IsTokenRevoked(_ context.Context, _ TokenType, token string) (bool, error) {
	_, ok := r.revoked[token]
	return ok, nil
}

// Test that validateClaims handles nil time fields without panicking.
func TestValidateClaims_NilTimeFields(t *testing.T) {
	claims := &TokenClaims{
		ID:        uuid.New(),
		Subject:   uuid.New(),
		SessionID: uuid.New(),
		Issuer:    "test-issuer",
		Audience:  []string{"test-audience"},
		TokenType: AccessToken,
		// Intentionally leave IssuedAt, ExpiresAt, NotBefore as nil
	}

	// This should return an error, not panic
	err := validateClaims(claims, "test-issuer", "test-audience", AccessToken, time.Now())
	if err == nil {
		t.Error("expected error for nil time fields, got nil")
	}
}

// Test that parsing a token without standard time claims leaves pointers nil.
func TestParseTokenWithoutTimeClaims(t *testing.T) {
	maker, err := NewTokenMaker(Config{
		Secret:   "test-secret-must-be-at-least-32-bytes",
		Issuer:   "test-issuer",
		Audience: "test-audience",
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Create a minimal token directly with MapClaims (bypassing our helpers)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti": uuid.New().String(),
		"sub": uuid.New().String(),
		"sid": uuid.New().String(),
		"iss": "test-issuer",
		"aud": []string{"test-audience"},
		"typ": "access",
	})
	tokenString, err := token.SignedString([]byte(maker.secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	// Parsing with our TokenClaims should work; verifyToken will call validateClaims
	_, err = maker.verifyToken(tokenString, AccessToken)
	if err == nil {
		t.Error("expected verification to fail for token without exp/nbf")
	}
}

// Test that RevokeAccessToken can revoke an expired token (the bug it previously
// failed at because jwt.ParseWithClaims validated time by default).
func TestRevokeAccessToken_ExpiredToken(t *testing.T) {
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Millisecond,
		RefreshExpiryDuration: time.Hour,
	}, repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Create a token that expires almost immediately
	tokenResp, err := maker.CreateAccessToken(context.Background(), uuid.New(), "test", []string{"user"}, uuid.New())
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	// Wait for the token to expire
	time.Sleep(5 * time.Millisecond)

	// RevokeAccessToken should succeed even though the token is expired
	err = maker.RevokeAccessToken(context.Background(), tokenResp.Token)
	if err != nil {
		t.Fatalf("expected RevokeAccessToken to succeed for expired token, got: %v", err)
	}

	// Verify the token was actually revoked
	revoked, err := repo.IsTokenRevoked(context.Background(), AccessToken, tokenResp.Token)
	if err != nil {
		t.Fatalf("IsTokenRevoked failed: %v", err)
	}
	if !revoked {
		t.Error("expected token to be revoked")
	}
}
