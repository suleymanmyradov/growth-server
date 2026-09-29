package jwt

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// mockRevocationRepo is a simple in-memory RevocationRepository for testing.
type mockRevocationRepo struct {
	revoked map[string]string
	rotated map[string]string
}

func newMockRevocationRepo() *mockRevocationRepo {
	return &mockRevocationRepo{revoked: make(map[string]string), rotated: make(map[string]string)}
}

func (r *mockRevocationRepo) MarkTokenRevoke(_ context.Context, _ TokenType, token string, _ time.Duration) error {
	r.revoked[token] = "1"
	return nil
}

func (r *mockRevocationRepo) IsTokenRevoked(_ context.Context, _ TokenType, token string) (bool, error) {
	_, ok := r.revoked[token]
	return ok, nil
}

func (r *mockRevocationRepo) MarkSessionRevoked(_ context.Context, sessionID string, _ time.Duration) error {
	r.revoked["session:"+sessionID] = "1"
	return nil
}

func (r *mockRevocationRepo) IsSessionRevoked(_ context.Context, sessionID string) (bool, error) {
	_, ok := r.revoked["session:"+sessionID]
	return ok, nil
}

func (r *mockRevocationRepo) ConsumeRefreshToken(_ context.Context, token string, marker string, _ time.Duration) (bool, error) {
	if _, ok := r.revoked[token]; ok {
		return false, nil
	}
	r.revoked[token] = marker
	return true, nil
}

func (r *mockRevocationRepo) UnconsumeRefreshToken(_ context.Context, token string, marker string) error {
	if r.revoked[token] == marker {
		delete(r.revoked, token)
	}
	return nil
}

func (r *mockRevocationRepo) StoreRotatedRefresh(_ context.Context, token string, newToken string, _ time.Duration) error {
	r.rotated[token] = newToken
	return nil
}

func (r *mockRevocationRepo) RotatedRefreshFor(_ context.Context, token string) (string, bool, error) {
	v, ok := r.rotated[token]
	return v, ok, nil
}

func (r *mockRevocationRepo) RefreshMarkerFor(_ context.Context, token string) (string, bool, error) {
	v, ok := r.revoked[token]
	return v, ok, nil
}

func (r *mockRevocationRepo) MarkUserRevoked(_ context.Context, userID string, revokedAt time.Time, _ time.Duration) error {
	r.revoked["user:"+userID] = fmt.Sprintf("%d", revokedAt.Unix())
	return nil
}

func (r *mockRevocationRepo) IsUserRevoked(_ context.Context, userID string, issuedAt time.Time) (bool, error) {
	v, ok := r.revoked["user:"+userID]
	if !ok {
		return false, nil
	}
	cutoff, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return false, err
	}
	return issuedAt.Unix() <= cutoff, nil
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
	privPEM, _ := testKeyPair(t)
	maker, err := NewTokenMaker(testECConfig(privPEM), nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// Create a minimal token directly with MapClaims (bypassing our helpers)
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"jti": uuid.New().String(),
		"sub": uuid.New().String(),
		"sid": uuid.New().String(),
		"iss": "test-issuer",
		"aud": []string{"test-audience"},
		"typ": "access",
	})
	tokenString, err := token.SignedString(maker.signingKey)
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
	privPEM, _ := testKeyPair(t)
	cfg := testECConfig(privPEM)
	cfg.AccessExpiryDuration = time.Millisecond
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(cfg, repo)
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
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
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
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
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

// TestRefreshSession_AtomicConsume — the core race fix: only ONE caller may
// consume a refresh token; the loser gets the winner's pair via the grace
// replay rather than minting a divergent one.
func TestRefreshSession_AtomicConsume(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	ctx := context.Background()
	sessionID := uuid.New()
	refreshResp, err := maker.CreateRefreshToken(ctx, uuid.New(), "u", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	first, err := maker.RefreshSession(ctx, refreshResp.Token)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if first.Replayed {
		t.Fatal("first refresh should not be a replay")
	}

	// Second call with the SAME old token replays the winner's pair.
	second, err := maker.RefreshSession(ctx, refreshResp.Token)
	if err != nil {
		t.Fatalf("grace replay failed: %v", err)
	}
	if !second.Replayed {
		t.Fatal("expected second call to replay the successor")
	}
	if second.RefreshToken.Token != first.RefreshToken.Token {
		t.Fatal("replay returned a different pair — divergent rotation")
	}
}

// TestRefreshSession_ReuseRevokesSession — a rotated token replayed after
// the grace window is theft: the whole session dies.
func TestRefreshSession_ReuseRevokesSession(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	ctx := context.Background()
	sessionID := uuid.New()
	refreshResp, err := maker.CreateRefreshToken(ctx, uuid.New(), "u", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
	if _, err := maker.RefreshSession(ctx, refreshResp.Token); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	// Simulate the grace window having passed: drop the rotated-pair entry and
	// backdate the consume marker beyond the 60s grace.
	delete(repo.rotated, refreshResp.Token)
	repo.revoked[refreshResp.Token] = fmt.Sprintf("%d|%s", time.Now().Add(-61*time.Second).Unix(), sessionID)

	_, err = maker.RefreshSession(ctx, refreshResp.Token)
	if err == nil {
		t.Fatal("replayed past-grace token must be rejected")
	}
	if !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("expected ErrSessionRevoked, got %v", err)
	}
	revoked, _ := repo.IsSessionRevoked(ctx, sessionID.String())
	if !revoked {
		t.Fatal("session should be revoked after reuse detection")
	}
}

// TestRefreshSession_PlainRevokedNotReuse — a token revoked by logout
// (marker "1", not a rotation marker) fails closed but does NOT escalate
// to session revocation (the session is already handled by logout).
func TestRefreshSession_PlainRevokedNotReuse(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	ctx := context.Background()
	sessionID := uuid.New()
	refreshResp, err := maker.CreateRefreshToken(ctx, uuid.New(), "u", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
	if err := maker.RevokeRefreshToken(ctx, refreshResp.Token); err != nil {
		t.Fatalf("revoke refresh token: %v", err)
	}

	_, err = maker.RefreshSession(ctx, refreshResp.Token)
	if err == nil {
		t.Fatal("revoked token must be rejected")
	}
	revoked, _ := repo.IsSessionRevoked(ctx, sessionID.String())
	if revoked {
		t.Fatal("plain token revocation must not escalate to session revocation")
	}
}

// TestRefreshSession_InFlightMarker — a consume marker with no stored pair
// inside the grace window returns ErrRotationInFlight (retryable), not a
// session-killing error.
func TestRefreshSession_InFlightMarker(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	ctx := context.Background()
	sessionID := uuid.New()
	refreshResp, err := maker.CreateRefreshToken(ctx, uuid.New(), "u", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	// Winner consumed but hasn't stored the pair yet.
	repo.revoked[refreshResp.Token] = fmt.Sprintf("%d|%s", time.Now().Unix(), sessionID)

	// Shrink the poll loop for the test by pre-expiring context quickly is
	// fragile — instead just let it poll (1.5s) once. Acceptable in CI.
	_, err = maker.RefreshSession(ctx, refreshResp.Token)
	if !errors.Is(err, ErrRotationInFlight) {
		t.Fatalf("expected ErrRotationInFlight, got %v", err)
	}
}

// Password reset/change kills every session via an issued-at cutoff:
// tokens minted before the marker are dead, tokens minted after live.
func TestRevokeAllUserSessions_Cutoff(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()

	accessResp, err := maker.CreateAccessToken(ctx, userID, "u", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}
	refreshResp, err := maker.CreateRefreshToken(ctx, userID, "u", []string{"user"}, sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	// Password reset happens now: same-second iat is <= cutoff, so the
	// pre-existing tokens die.
	if err := maker.RevokeAllUserSessions(ctx, userID); err != nil {
		t.Fatalf("revoke all user sessions: %v", err)
	}

	if _, err := maker.VerifyAccessToken(ctx, accessResp.Token); err == nil {
		t.Fatal("expected pre-cutoff access token to be rejected")
	}
	if _, err := maker.RefreshSession(ctx, refreshResp.Token); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("expected ErrSessionRevoked for pre-cutoff refresh token, got %v", err)
	}

	// Simulate the reset having happened a second ago: a fresh login then
	// mints a token with iat > cutoff, which must verify.
	if err := maker.repo.MarkUserRevoked(ctx, userID.String(), time.Now().Add(-time.Second), time.Hour); err != nil {
		t.Fatalf("backdate cutoff: %v", err)
	}
	// A login after the cutoff still works.
	newAccess, err := maker.CreateAccessToken(ctx, userID, "u", []string{"user"}, uuid.New())
	if err != nil {
		t.Fatalf("create post-cutoff access token: %v", err)
	}
	if _, err := maker.VerifyAccessToken(ctx, newAccess.Token); err != nil {
		t.Fatalf("post-cutoff access token should verify, got %v", err)
	}
}
