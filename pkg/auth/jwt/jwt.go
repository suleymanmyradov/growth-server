package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
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
	}

	return claims, nil
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
	oldClaims, err := tm.VerifyRefreshToken(ctx, oldToken)
	if err != nil {
		return nil, fmt.Errorf("verify old token: %w", err)
	}

	if tm.repo != nil && oldClaims.ExpiresAt != nil {
		ttl := time.Until(oldClaims.ExpiresAt.Time)
		if ttl > 0 {
			if err := tm.repo.MarkTokenRevoke(ctx, RefreshToken, oldToken, ttl); err != nil {
				return nil, fmt.Errorf("revoke old token: %w", err)
			}
		}
	}

	return tm.CreateRefreshToken(ctx, oldClaims.Subject, oldClaims.Username, oldClaims.Roles, oldClaims.SessionID)
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
