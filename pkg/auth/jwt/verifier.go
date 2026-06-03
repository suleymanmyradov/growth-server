package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// KeyFunc resolves the public key for a given key ID (kid).
// Return nil to indicate the key is unknown.
type KeyFunc interface {
	// GetKey returns the public key for the given kid and algorithm.
	// Supported key types: *rsa.PublicKey, *ecdsa.PublicKey, ed25519.PublicKey.
	GetKey(kid, alg string) (interface{}, error)
}

// StaticKeyFunc wraps a single public key for deployments that do not use JWKS.
type StaticKeyFunc struct {
	Key interface{}
}

// GetKey returns the static key, ignoring kid and alg.
func (s *StaticKeyFunc) GetKey(_, _ string) (interface{}, error) {
	if s.Key == nil {
		return nil, fmt.Errorf("static key is nil")
	}
	return s.Key, nil
}

// Verifier verifies access tokens using asymmetric cryptography (RS256, ES256, EdDSA).
// It satisfies the mdpropagate.TokenVerifier interface so downstream services can
// verify tokens without possessing the signing secret.
type Verifier struct {
	issuer    string
	audience  string
	keyFunc   KeyFunc
	leeway    time.Duration
}

// VerifierConfig holds configuration for the asymmetric token verifier.
type VerifierConfig struct {
	Issuer   string
	Audience string
	KeyFunc  KeyFunc
	// Leeway defaults to DefaultLeeway if zero.
	Leeway time.Duration
}

// NewVerifier creates an asymmetric token verifier.
func NewVerifier(cfg VerifierConfig) (*Verifier, error) {
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("verifier issuer is required")
	}
	if cfg.Audience == "" {
		return nil, fmt.Errorf("verifier audience is required")
	}
	if cfg.KeyFunc == nil {
		return nil, fmt.Errorf("verifier keyFunc is required")
	}
	leeway := cfg.Leeway
	if leeway == 0 {
		leeway = DefaultLeeway
	}
	return &Verifier{
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		keyFunc:  cfg.KeyFunc,
		leeway:   leeway,
	}, nil
}

// VerifyAccessToken validates an access token using the configured public key(s).
func (v *Verifier) VerifyAccessToken(_ context.Context, tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		alg, ok := token.Header["alg"].(string)
		if !ok {
			return nil, ErrInvalidToken
		}
		kid, _ := token.Header["kid"].(string)
		return v.keyFunc.GetKey(kid, alg)
	}, jwt.WithLeeway(v.leeway))
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
