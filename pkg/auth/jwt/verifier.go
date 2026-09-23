package jwt

import (
	"context"
	"crypto/elliptic"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Verifier verifies access tokens without holding any signing credential.
// It satisfies the mdpropagate.TokenVerifier interface so downstream services
// can verify tokens with only the public key — a leaked verifier config
// cannot mint tokens.
type Verifier struct {
	issuer   string
	audience string
	resolver keyResolver
	leeway   time.Duration
}

// NewVerifier creates a verify-only token checker from the shared Config.
// It requires Config.PublicKey (ES256). PrivateKey is ignored — verifiers
// never sign.
func NewVerifier(cfg Config) (*Verifier, error) {
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("config.Issuer is required")
	}
	if cfg.Audience == "" {
		return nil, fmt.Errorf("config.Audience is required")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("config.PublicKey is required")
	}

	v := &Verifier{
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		leeway:   DefaultLeeway,
	}
	key, err := ParsePublicKeyPEM(cfg.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("config.PublicKey: %w", err)
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("config.PublicKey: ES256 requires a P-256 key")
	}
	v.resolver.publicKey = key
	return v, nil
}

// VerifyAccessToken validates an access token's signature, issuer, audience,
// type, and time claims.
func (v *Verifier) VerifyAccessToken(_ context.Context, tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, v.resolver.keyfunc,
		jwt.WithValidMethods(v.resolver.validMethods()), jwt.WithLeeway(v.leeway))
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	now := time.Now()
	if err := validateClaims(claims, v.issuer, v.audience, AccessToken, now); err != nil {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// MustVerifyAccessToken is a convenience wrapper that returns an error if verification fails.
func (v *Verifier) MustVerifyAccessToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	claims, err := v.VerifyAccessToken(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	return claims, nil
}
