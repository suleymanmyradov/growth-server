package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// DefaultLeeway is the clock skew tolerance for time-based claims.
const DefaultLeeway = 30 * time.Second

// ErrInvalidToken is returned for all token verification failures to prevent
// information leakage about which specific check failed.
var ErrInvalidToken = fmt.Errorf("invalid token")

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

type TokenClaims struct {
	ID        uuid.UUID        `json:"jti"`
	Subject   uuid.UUID        `json:"sub"`
	SessionID uuid.UUID        `json:"sid"`
	Username  string           `json:"usr,omitempty"`
	Roles     []string         `json:"rls,omitempty"`
	Issuer    string           `json:"iss"`
	Audience  []string         `json:"aud"`
	IssuedAt  *jwt.NumericDate `json:"iat"`
	ExpiresAt *jwt.NumericDate `json:"exp"`
	NotBefore *jwt.NumericDate `json:"nbf"`
	TokenType TokenType        `json:"typ"`
}

func (c *TokenClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return c.ExpiresAt, nil
}

func (c *TokenClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return c.IssuedAt, nil
}

func (c *TokenClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return c.NotBefore, nil
}

func (c *TokenClaims) GetIssuer() (string, error) {
	return c.Issuer, nil
}

func (c *TokenClaims) GetSubject() (string, error) {
	return c.Subject.String(), nil
}

func (c *TokenClaims) GetAudience() (jwt.ClaimStrings, error) {
	return c.Audience, nil
}

type TokenResponse struct {
	Token     string
	ExpiresAt time.Time
}

type RevocationRepository interface {
	MarkTokenRevoke(ctx context.Context, tokenType TokenType, token string, ttl time.Duration) error
	IsTokenRevoked(ctx context.Context, tokenType TokenType, token string) (bool, error)
	MarkSessionRevoked(ctx context.Context, sessionID string, ttl time.Duration) error
	IsSessionRevoked(ctx context.Context, sessionID string) (bool, error)
	// ConsumeRefreshToken atomically claims a refresh token (SET NX semantics
	// on the revoked key). Returns true only for the winning caller.
	ConsumeRefreshToken(ctx context.Context, token string, marker string, ttl time.Duration) (bool, error)
	// UnconsumeRefreshToken rolls back a consume iff the marker still matches.
	UnconsumeRefreshToken(ctx context.Context, token string, marker string) error
	// StoreRotatedRefresh caches the replacement for a consumed token for the
	// grace window; RotatedRefreshFor reads it back.
	StoreRotatedRefresh(ctx context.Context, token string, newToken string, ttl time.Duration) error
	RotatedRefreshFor(ctx context.Context, token string) (string, bool, error)
	// RefreshMarkerFor returns the consumed-marker stored on the revoked key
	// ("<unixTs>|<sessionID>" for rotation consumes, "1" for plain revokes).
	RefreshMarkerFor(ctx context.Context, token string) (string, bool, error)
	// MarkUserRevoked stores a cutoff instant for a user — tokens issued at
	// or before it are dead. Used by password reset/change to kill every
	// session without enumerating session IDs.
	MarkUserRevoked(ctx context.Context, userID string, revokedAt time.Time, ttl time.Duration) error
	IsUserRevoked(ctx context.Context, userID string, issuedAt time.Time) (bool, error)
}

// keyResolver maps a token's signing method to the credential that verifies
// it. Only ES256 is accepted: tokens resolve to the ECDSA public key and any
// other signing method — including HS256 — is rejected, so the PEM public
// key can never be abused as an HMAC secret (algorithm-confusion).
type keyResolver struct {
	publicKey *ecdsa.PublicKey
}

func (r keyResolver) validMethods() []string {
	if r.publicKey == nil {
		return nil
	}
	return []string{"ES256"}
}

func (r keyResolver) keyfunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodECDSA); ok && r.publicKey != nil {
		return r.publicKey, nil
	}
	return nil, ErrInvalidToken
}

type TokenMaker struct {
	signingKey    *ecdsa.PrivateKey
	signingKeyID  string
	resolver      keyResolver
	issuer        string
	audience      string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	repo          RevocationRepository
}

type Config struct {
	// PrivateKey is a PEM-encoded ECDSA P-256 private key (PKCS#8 or SEC1)
	// used to sign tokens with ES256. Only token-issuing services (auth,
	// adminway) should have it. Newlines may be escaped ("\n") for env vars.
	PrivateKey string `json:",optional" secret:"true"`
	// PublicKey is the PEM-encoded ECDSA P-256 public key used to verify
	// ES256 tokens. Safe to distribute to every verifying service. When
	// PrivateKey is set the public half is derived from it instead.
	PublicKey             string        `json:",optional"`
	Issuer                string        `json:",optional"`
	Audience              string        `json:",optional"`
	AccessExpiryDuration  time.Duration `json:",optional"`
	RefreshExpiryDuration time.Duration `json:",optional"`
}

func NewTokenMaker(cfg Config, repo RevocationRepository) (*TokenMaker, error) {
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("config.Issuer is required")
	}
	if cfg.Audience == "" {
		return nil, fmt.Errorf("config.Audience is required")
	}
	if cfg.PrivateKey == "" && cfg.PublicKey == "" {
		return nil, fmt.Errorf("config requires one of PrivateKey, PublicKey")
	}

	tm := &TokenMaker{
		issuer:        cfg.Issuer,
		audience:      cfg.Audience,
		accessExpiry:  cfg.AccessExpiryDuration,
		refreshExpiry: cfg.RefreshExpiryDuration,
		repo:          repo,
	}

	if cfg.PrivateKey != "" {
		key, err := ParsePrivateKeyPEM(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("config.PrivateKey: %w", err)
		}
		if key.Curve != elliptic.P256() {
			return nil, fmt.Errorf("config.PrivateKey: ES256 requires a P-256 key")
		}
		tm.signingKey = key
		tm.signingKeyID = KeyID(&key.PublicKey)
		// A signer verifies exactly what it signs — the public half always
		// comes from the private key, ignoring config.PublicKey.
		tm.resolver.publicKey = &key.PublicKey
	} else if cfg.PublicKey != "" {
		key, err := ParsePublicKeyPEM(cfg.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("config.PublicKey: %w", err)
		}
		if key.Curve != elliptic.P256() {
			return nil, fmt.Errorf("config.PublicKey: ES256 requires a P-256 key")
		}
		tm.resolver.publicKey = key
	}

	return tm, nil
}

// signClaims serializes claims into a signed JWT with ES256. It requires a
// configured private key — there is no symmetric fallback.
func (tm *TokenMaker) signClaims(claims *TokenClaims) (string, error) {
	if tm.signingKey == nil {
		return "", fmt.Errorf("no signing credential configured")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = tm.signingKeyID
	tokenString, err := token.SignedString(tm.signingKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return tokenString, nil
}

func (tm *TokenMaker) CreateAccessToken(_ context.Context, userID uuid.UUID, username string, roles []string, sessionID uuid.UUID) (*TokenResponse, error) {
	now := time.Now()
	expiresAt := now.Add(tm.accessExpiry)

	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   userID,
		SessionID: sessionID,
		Username:  username,
		Roles:     roles,
		Issuer:    tm.issuer,
		Audience:  []string{tm.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		NotBefore: jwt.NewNumericDate(now),
		TokenType: AccessToken,
	}

	tokenString, err := tm.signClaims(&claims)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}, nil
}

func (tm *TokenMaker) CreateRefreshToken(_ context.Context, userID uuid.UUID, username string, roles []string, sessionID uuid.UUID) (*TokenResponse, error) {
	now := time.Now()
	expiresAt := now.Add(tm.refreshExpiry)

	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   userID,
		SessionID: sessionID,
		Username:  username,
		Roles:     roles,
		Issuer:    tm.issuer,
		Audience:  []string{tm.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		NotBefore: jwt.NewNumericDate(now),
		TokenType: RefreshToken,
	}

	tokenString, err := tm.signClaims(&claims)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}, nil
}

func (tm *TokenMaker) VerifyAccessToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	claims, err := tm.verifyToken(tokenString, AccessToken)
	if err != nil {
		return nil, err
	}

	if tm.repo != nil {
		revoked, err := tm.repo.IsTokenRevoked(ctx, AccessToken, tokenString)
		if err != nil {
			return nil, fmt.Errorf("check revocation: %w", err)
		}
		if revoked {
			return nil, fmt.Errorf("token revoked")
		}
		if err := tm.checkUserRevoked(ctx, claims); err != nil {
			return nil, err
		}
	}

	return claims, nil
}

func (tm *TokenMaker) VerifyRefreshToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	claims, err := tm.verifyToken(tokenString, RefreshToken)
	if err != nil {
		return nil, err
	}

	if tm.repo != nil {
		// Check session revocation first: this is the key protection against a
		// copied refresh token surviving logout. Logout revokes the entire
		// session by session ID, so any refresh token (including copies) issued
		// for that session is rejected here.
		revoked, err := tm.repo.IsSessionRevoked(ctx, claims.SessionID.String())
		if err != nil {
			return nil, fmt.Errorf("check session revocation: %w", err)
		}
		if revoked {
			return nil, fmt.Errorf("session revoked")
		}

		// Defense-in-depth: also check token-value revocation.
		revoked, err = tm.repo.IsTokenRevoked(ctx, RefreshToken, tokenString)
		if err != nil {
			return nil, fmt.Errorf("check revocation: %w", err)
		}
		if revoked {
			return nil, fmt.Errorf("token revoked")
		}
		if err := tm.checkUserRevoked(ctx, claims); err != nil {
			return nil, err
		}
	}

	return claims, nil
}

// checkUserRevoked rejects tokens issued at/before the user's revocation
// cutoff (password reset/change kills every session this way).
func (tm *TokenMaker) checkUserRevoked(ctx context.Context, claims *TokenClaims) error {
	if claims.IssuedAt == nil || claims.Subject == uuid.Nil {
		return nil
	}
	revoked, err := tm.repo.IsUserRevoked(ctx, claims.Subject.String(), claims.IssuedAt.Time)
	if err != nil {
		return fmt.Errorf("check user revocation: %w", err)
	}
	if revoked {
		return fmt.Errorf("user sessions revoked")
	}
	return nil
}

// RevokeAllUserSessions invalidates every token (access + refresh) the user
// currently holds, without needing to enumerate session IDs. New logins
// still work — tokens minted after this instant pass the cutoff check.
func (tm *TokenMaker) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	if tm.repo == nil {
		return fmt.Errorf("revocation not enabled")
	}
	return tm.repo.MarkUserRevoked(ctx, userID.String(), time.Now(), tm.refreshExpiry)
}

func (tm *TokenMaker) verifyToken(tokenString string, expectedType TokenType) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, tm.resolver.keyfunc,
		jwt.WithValidMethods(tm.resolver.validMethods()), jwt.WithLeeway(DefaultLeeway))
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	now := time.Now()
	if err := validateClaims(claims, tm.issuer, tm.audience, expectedType, now); err != nil {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (tm *TokenMaker) RevokeAccessToken(ctx context.Context, tokenString string) error {
	if tm.repo == nil {
		return fmt.Errorf("revocation not enabled")
	}

	// Parse token without claims validation to allow revocation of expired tokens.
	// Signature and algorithm are still verified; issuer/audience/type are checked manually below.
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, tm.resolver.keyfunc,
		jwt.WithValidMethods(tm.resolver.validMethods()), jwt.WithoutClaimsValidation())
	if err != nil || !token.Valid {
		return ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return ErrInvalidToken
	}

	// Validate issuer and audience (but not time)
	if claims.Issuer != tm.issuer {
		return ErrInvalidToken
	}

	validAudience := false
	for _, aud := range claims.Audience {
		if aud == tm.audience {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return ErrInvalidToken
	}

	if claims.TokenType != AccessToken {
		return ErrInvalidToken
	}

	if claims.ExpiresAt == nil {
		return ErrInvalidToken
	}

	// Use the token's expiry time, capped at a minimum to prevent replay attacks
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < time.Minute {
		ttl = time.Minute
	}

	return tm.repo.MarkTokenRevoke(ctx, AccessToken, tokenString, ttl)
}

// RevokeRefreshToken revokes a refresh token by value. Used during logout for
// defense-in-depth alongside session-level revocation.
func (tm *TokenMaker) RevokeRefreshToken(ctx context.Context, tokenString string) error {
	if tm.repo == nil {
		return fmt.Errorf("revocation not enabled")
	}

	// Parse without time validation to allow revoking expired tokens.
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, tm.resolver.keyfunc,
		jwt.WithValidMethods(tm.resolver.validMethods()), jwt.WithoutClaimsValidation())
	if err != nil || !token.Valid {
		return ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return ErrInvalidToken
	}

	if claims.Issuer != tm.issuer {
		return ErrInvalidToken
	}

	validAudience := false
	for _, aud := range claims.Audience {
		if aud == tm.audience {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return ErrInvalidToken
	}

	if claims.TokenType != RefreshToken {
		return ErrInvalidToken
	}

	if claims.ExpiresAt == nil {
		return ErrInvalidToken
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < time.Minute {
		ttl = time.Minute
	}

	return tm.repo.MarkTokenRevoke(ctx, RefreshToken, tokenString, ttl)
}

// RevokeSession marks an entire session as revoked. All refresh tokens for that
// session will be rejected on the next refresh attempt, even if the token value
// itself hasn't been revoked. The TTL should cover the refresh token's max
// remaining lifetime so the entry is cleaned up automatically.
func (tm *TokenMaker) RevokeSession(ctx context.Context, sessionID uuid.UUID, ttl time.Duration) error {
	if tm.repo == nil {
		return fmt.Errorf("revocation not enabled")
	}
	if ttl < time.Minute {
		ttl = time.Minute
	}
	return tm.repo.MarkSessionRevoked(ctx, sessionID.String(), ttl)
}

// IsSessionRevoked checks whether a session has been revoked.
func (tm *TokenMaker) IsSessionRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	if tm.repo == nil {
		return false, nil
	}
	return tm.repo.IsSessionRevoked(ctx, sessionID.String())
}

func (tm *TokenMaker) RotateRefreshToken(ctx context.Context, oldToken string) (*TokenResponse, error) {
	result, err := tm.RefreshSession(ctx, oldToken)
	if err != nil {
		return nil, err
	}
	return result.RefreshToken, nil
}

// ErrRotationInFlight is returned when a refresh token was just consumed by
// a concurrent request whose rotated pair hasn't been stored yet — the
// caller should retry shortly rather than treating the session as dead.
var ErrRotationInFlight = fmt.Errorf("refresh token rotation in flight")

// ErrSessionRevoked is returned when the session the token belongs to was
// revoked (logout, password change, reuse detection).
var ErrSessionRevoked = fmt.Errorf("session revoked")

// rotationGraceTTL is how long a just-rotated token replays to its successor
// pair. It absorbs legitimate client-side races (parallel requests that all
// snapshotted the pre-rotation token) without weakening reuse detection.
const rotationGraceTTL = 60 * time.Second

// rotationPollInterval/Attempts cover the gap between a winner's consume
// marker landing and its rotated pair being stored (~one Redis RTT).
const (
	rotationPollInterval = 100 * time.Millisecond
	rotationPollAttempts = 15
)

// RefreshSessionResult carries everything the refresh RPC needs: the claims
// of the presented token (for user lookup) and the refresh token to return
// (new, or the replayed successor for stragglers inside the grace window).
type RefreshSessionResult struct {
	Claims       *TokenClaims
	RefreshToken *TokenResponse
	// Replayed is true when the presented token had already been consumed and
	// the stored successor was returned — i.e. a concurrent rotation won.
	Replayed bool
}

// RefreshSession performs an atomic refresh-token rotation.
//
// The old flow was check-then-set (EXISTS then SET): two concurrent refreshes
// of the same token both passed the revoked check and both minted pairs, and
// a replayed stolen token was indistinguishable from a legit retry. This
// version consumes the token with SET NX, caches the successor pair for a
// 60s grace window so stragglers get the same pair, and treats a consumed
// token with no cached successor past the grace window as REUSE — which
// revokes the whole session.
func (tm *TokenMaker) RefreshSession(ctx context.Context, oldToken string) (*RefreshSessionResult, error) {
	// Verify signature + claims only — the revocation check happens atomically
	// below (a consumed token must still parse so grace replay can find its
	// successor).
	oldClaims, err := tm.verifyToken(oldToken, RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("verify old token: %w", err)
	}

	if tm.repo == nil {
		// No revocation store — rotate without tracking (dev/test fallback).
		pair, err := tm.CreateRefreshToken(ctx, oldClaims.Subject, oldClaims.Username, oldClaims.Roles, oldClaims.SessionID)
		if err != nil {
			return nil, err
		}
		return &RefreshSessionResult{Claims: oldClaims, RefreshToken: pair}, nil
	}

	if oldClaims.SessionID != uuid.Nil {
		revoked, err := tm.repo.IsSessionRevoked(ctx, oldClaims.SessionID.String())
		if err != nil {
			return nil, fmt.Errorf("check session revocation: %w", err)
		}
		if revoked {
			return nil, ErrSessionRevoked
		}
	}

	// User-level cutoff (password reset/change): tokens issued before the
	// cutoff must not rotate into fresh sessions.
	if err := tm.checkUserRevoked(ctx, oldClaims); err != nil {
		return nil, ErrSessionRevoked
	}

	ttl := time.Minute
	if oldClaims.ExpiresAt != nil {
		if remaining := time.Until(oldClaims.ExpiresAt.Time); remaining > ttl {
			ttl = remaining
		}
	}

	marker := fmt.Sprintf("%d|%s", time.Now().Unix(), oldClaims.SessionID)
	consumed, err := tm.repo.ConsumeRefreshToken(ctx, oldToken, marker, ttl)
	if err != nil {
		return nil, fmt.Errorf("consume old token: %w", err)
	}

	if consumed {
		pair, err := tm.CreateRefreshToken(ctx, oldClaims.Subject, oldClaims.Username, oldClaims.Roles, oldClaims.SessionID)
		if err != nil {
			return nil, err
		}
		if err := tm.repo.StoreRotatedRefresh(ctx, oldToken, pair.Token, rotationGraceTTL); err != nil {
			// Roll back the consume so a straggler (or the client's retry)
			// re-attempts the rotation rather than hitting reuse detection.
			_ = tm.repo.UnconsumeRefreshToken(ctx, oldToken, marker)
			return nil, fmt.Errorf("store rotated pair: %w", err)
		}
		return &RefreshSessionResult{Claims: oldClaims, RefreshToken: pair}, nil
	}

	// The token was already consumed — either a concurrent rotation is in
	// flight (pair not yet stored) or this is a replay.
	for i := 0; i < rotationPollAttempts; i++ {
		successor, ok, err := tm.repo.RotatedRefreshFor(ctx, oldToken)
		if err != nil {
			return nil, fmt.Errorf("check rotated pair: %w", err)
		}
		if ok {
			return &RefreshSessionResult{
				Claims:       oldClaims,
				RefreshToken: &TokenResponse{Token: successor, ExpiresAt: claimsExpiry(successor, oldClaims.ExpiresAt)},
				Replayed:     true,
			}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(rotationPollInterval):
		}
	}

	// No successor appeared. A young rotation marker means the winner is
	// still mid-flight (or crashed) — tell the caller to retry rather than
	// punish the session. An old marker (or a plain "1" from logout) means
	// the token is being replayed past its grace window: revoke the session.
	markerVal, markerOK, err := tm.repo.RefreshMarkerFor(ctx, oldToken)
	if err != nil {
		return nil, fmt.Errorf("check consume marker: %w", err)
	}
	if !markerOK {
		// Marker already gone (token TTL expired mid-poll, etc.) — can't
		// confirm reuse, so reject without punishing the session.
		return nil, fmt.Errorf("token revoked")
	}
	if consumedAt, ok := parseRotationMarker(markerVal); ok {
		if time.Since(consumedAt) < rotationGraceTTL {
			return nil, ErrRotationInFlight
		}
	} else {
		// Plain revocation marker (logout / password reset) — not reuse.
		return nil, fmt.Errorf("token revoked")
	}

	// Reuse detected: a rotated token presented after its grace window.
	if oldClaims.SessionID != uuid.Nil {
		_ = tm.repo.MarkSessionRevoked(ctx, oldClaims.SessionID.String(), ttl)
	}
	return nil, ErrSessionRevoked
}

// parseRotationMarker extracts the consume timestamp from a
// "<unixTs>|<sessionID>" marker. Returns false for non-rotation markers
// (e.g. "1" written by plain revocation).
func parseRotationMarker(marker string) (time.Time, bool) {
	pipe := strings.IndexByte(marker, '|')
	if pipe <= 0 {
		return time.Time{}, false
	}
	ts, err := strconv.ParseInt(marker[:pipe], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(ts, 0), true
}

// claimsExpiry decodes the exp claim of a token we minted ourselves (signature
// already trusted), falling back to the old token's expiry when parsing fails.
func claimsExpiry(token string, fallback *jwt.NumericDate) time.Time {
	claims := &TokenClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err == nil && claims.ExpiresAt != nil {
		return claims.ExpiresAt.Time
	}
	if fallback != nil {
		return fallback.Time
	}
	return time.Now().Add(rotationGraceTTL)
}

// Note: This JWT package does not spawn any background goroutines.
// All operations (CreateAccessToken, CreateRefreshToken, VerifyAccessToken, etc.) are synchronous.
// No shutdown hooks are needed for cleanup.

// validateClaims performs common claim validation for both TokenMaker and Verifier.
// It returns ErrInvalidToken for any failure to prevent information leakage.
func validateClaims(claims *TokenClaims, issuer, audience string, expectedType TokenType, now time.Time) error {
	if claims.Issuer != issuer {
		return ErrInvalidToken
	}

	validAudience := false
	for _, aud := range claims.Audience {
		if aud == audience {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return ErrInvalidToken
	}

	if claims.TokenType != expectedType {
		return ErrInvalidToken
	}

	if claims.NotBefore == nil || claims.ExpiresAt == nil {
		return ErrInvalidToken
	}
	if now.Before(claims.NotBefore.Add(-DefaultLeeway)) {
		return ErrInvalidToken
	}
	if now.After(claims.ExpiresAt.Add(DefaultLeeway)) {
		return ErrInvalidToken
	}

	return nil
}
