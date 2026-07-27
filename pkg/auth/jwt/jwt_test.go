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

func (r *mockRevocationRepo) MarkSessionRevoked(_ context.Context, sessionID string, _ time.Duration) error {
	r.revoked["session:"+sessionID] = struct{}{}
	return nil
}

func (r *mockRevocationRepo) IsSessionRevoked(_ context.Context, sessionID string) (bool, error) {
	_, ok := r.revoked["session:"+sessionID]
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

// TestRefreshAfterLogout_RejectedBySessionRevocation proves the critical
// security property that a copied refresh token cannot refresh after logout.
// Logout revokes the entire session (by session ID), so even if an attacker
// copied the refresh token value before logout, the session revocation check
// in VerifyRefreshToken blocks the refresh attempt.
func TestRefreshAfterLogout_RejectedBySessionRevocation(t *testing.T) {
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	sessionID := uuid.New()
	userID := uuid.New()

	// Issue a refresh token for this session.
	refreshResp, err := maker.CreateRefreshToken(context.Background(), userID, "test-user", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	// Verify the refresh token is valid before logout.
	claims, err := maker.VerifyRefreshToken(context.Background(), refreshResp.Token)
	if err != nil {
		t.Fatalf("verify refresh token before logout: %v", err)
	}
	if claims.SessionID != sessionID {
		t.Fatalf("expected session ID %s, got %s", sessionID, claims.SessionID)
	}

	// Logout: revoke the entire session by session ID (as logoutLogic does).
	err = maker.RevokeSession(context.Background(), sessionID, time.Hour)
	if err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	// A copied refresh token (same value) must now be rejected because the
	// session is revoked, even though the token value itself is not revoked.
	_, err = maker.VerifyRefreshToken(context.Background(), refreshResp.Token)
	if err == nil {
		t.Fatal("expected refresh token to be rejected after session revocation, but VerifyRefreshToken succeeded")
	}

	// Also verify via IsSessionRevoked for clarity.
	revoked, err := maker.IsSessionRevoked(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("IsSessionRevoked failed: %v", err)
	}
	if !revoked {
		t.Error("expected session to be revoked")
	}
}

// TestRefreshAfterLogout_RejectedByTokenRevocation proves that defense-in-depth
// token-value revocation also blocks refresh, even if the session check were
// somehow bypassed.
func TestRefreshAfterLogout_RejectedByTokenRevocation(t *testing.T) {
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(Config{
		Secret:                "test-secret-must-be-at-least-32-bytes",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}, repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	sessionID := uuid.New()
	refreshResp, err := maker.CreateRefreshToken(context.Background(), uuid.New(), "test-user", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	// Revoke the refresh token by value (defense-in-depth, as logoutLogic does
	// when the refresh token is provided).
	err = maker.RevokeRefreshToken(context.Background(), refreshResp.Token)
	if err != nil {
		t.Fatalf("revoke refresh token: %v", err)
	}

	// The refresh token must now be rejected by value revocation.
	_, err = maker.VerifyRefreshToken(context.Background(), refreshResp.Token)
	if err == nil {
		t.Fatal("expected refresh token to be rejected after token-value revocation, but VerifyRefreshToken succeeded")
	}
}
