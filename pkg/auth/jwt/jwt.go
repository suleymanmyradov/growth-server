package jwt

import (
	"context"
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
	ID        uuid.UUID `json:"jti"`
	Subject   uuid.UUID `json:"sub"`
	SessionID uuid.UUID `json:"sid"`
	Username  string    `json:"usr,omitempty"`
	Roles     []string  `json:"rls,omitempty"`
	Issuer    string    `json:"iss"`
	Audience  []string  `json:"aud"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
	NotBefore time.Time `json:"nbf"`
	TokenType TokenType `json:"typ"`
}

func (c *TokenClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(c.ExpiresAt), nil
}

func (c *TokenClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(c.IssuedAt), nil
}

func (c *TokenClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(c.NotBefore), nil
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
}

type TokenMaker struct {
	secret        string
	issuer        string
	audience      string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	repo          RevocationRepository
}

type Config struct {
	Secret                string        `json:",optional" secret:"true"`
	Issuer                string        `json:",optional"`
	Audience              string        `json:",optional"`
	AccessExpiryDuration  time.Duration `json:",optional"`
	RefreshExpiryDuration time.Duration `json:",optional"`
}

func NewTokenMaker(cfg Config, repo RevocationRepository) (*TokenMaker, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("config.Secret is required")
	}
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("config.Issuer is required")
	}
	if cfg.Audience == "" {
		return nil, fmt.Errorf("config.Audience is required")
	}

	return &TokenMaker{
		secret:        cfg.Secret,
		issuer:        cfg.Issuer,
		audience:      cfg.Audience,
		accessExpiry:  cfg.AccessExpiryDuration,
		refreshExpiry: cfg.RefreshExpiryDuration,
		repo:          repo,
	}, nil
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
		IssuedAt:  now,
		ExpiresAt: expiresAt,
		NotBefore: now,
		TokenType: AccessToken,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	tokenString, err := token.SignedString([]byte(tm.secret))
	if err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
	}

	return &TokenResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}, nil
}

func (tm *TokenMaker) CreateRefreshToken(_ context.Context, userID uuid.UUID, username string, sessionID uuid.UUID) (*TokenResponse, error) {
	now := time.Now()
	expiresAt := now.Add(tm.refreshExpiry)

	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   userID,
		SessionID: sessionID,
		Username:  username,
		Issuer:    tm.issuer,
		Audience:  []string{tm.audience},
		IssuedAt:  now,
		ExpiresAt: expiresAt,
		NotBefore: now,
		TokenType: RefreshToken,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	tokenString, err := token.SignedString([]byte(tm.secret))
	if err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
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
		revoked, err := tm.repo.IsTokenRevoked(ctx, RefreshToken, tokenString)
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
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(tm.secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithLeeway(DefaultLeeway))
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

	// Parse token without time validation to allow revocation of expired tokens
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(tm.secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithLeeway(DefaultLeeway))
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

	// Use the token's expiry time, capped at a minimum to prevent replay attacks
	ttl := time.Until(claims.ExpiresAt)
	if ttl < time.Minute {
		ttl = time.Minute
	}

	return tm.repo.MarkTokenRevoke(ctx, AccessToken, tokenString, ttl)
}

func (tm *TokenMaker) RotateRefreshToken(ctx context.Context, oldToken string) (*TokenResponse, error) {
	oldClaims, err := tm.VerifyRefreshToken(ctx, oldToken)
	if err != nil {
		return nil, fmt.Errorf("verify old token: %w", err)
	}

	if tm.repo != nil {
		ttl := time.Until(oldClaims.ExpiresAt)
		if ttl > 0 {
			if err := tm.repo.MarkTokenRevoke(ctx, RefreshToken, oldToken, ttl); err != nil {
				return nil, fmt.Errorf("revoke old token: %w", err)
			}
		}
	}

	return tm.CreateRefreshToken(ctx, oldClaims.Subject, oldClaims.Username, oldClaims.SessionID)
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

	if now.Before(claims.NotBefore.Add(-DefaultLeeway)) {
		return ErrInvalidToken
	}
	if now.After(claims.ExpiresAt.Add(DefaultLeeway)) {
		return ErrInvalidToken
	}

	return nil
}
