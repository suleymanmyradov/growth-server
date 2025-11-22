package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type AuthConfig struct {
	JwtSecretKey       string        `json:"jwt_secret_key"`
	AccessTokenExpiry  time.Duration `json:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `json:"refresh_token_expiry"`
	ResetTokenExpiry   time.Duration `json:"reset_token_expiry"`
}

type ValidateTokenResponse struct {
	Valid    bool   `json:"valid"`
	UserId   string `json:"user_id"`
	Username string `json:"username"`
}

type authManager struct {
	config AuthConfig
}

func NewAuthManager(cfg AuthConfig) *authManager {
	return &authManager{config: cfg}
}

// GenerateAccessToken creates a new JWT access token
func (a *authManager) GenerateAccessToken(userID, username, email string) (string, error) {
	claims := &TokenClaims{
		UserId:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(a.config.AccessTokenExpiry)),
			Issuer:    "growth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.JwtSecretKey))
}

// GenerateRefreshToken creates a new refresh token
func (a *authManager) GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateResetToken creates a new password reset token
func (a *authManager) GenerateResetToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate reset token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ParseToken validates and parses an access token
func (a *authManager) ParseToken(accessToken string) (*TokenClaims, error) {
	var claims TokenClaims

	token, err := jwt.ParseWithClaims(accessToken, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JwtSecretKey), nil
	})

	if err != nil {
		logx.Errorf("Failed to parse token: %v", err)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return &claims, nil
}

// ValidateToken checks if a token is valid and returns user info
func (a *authManager) ValidateToken(accessToken string) (*ValidateTokenResponse, error) {
	claims, err := a.ParseToken(accessToken)
	if err != nil {
		return &ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	return &ValidateTokenResponse{
		Valid:    true,
		UserId:   claims.UserId,
		Username: claims.Username,
	}, nil
}

// HashPassword creates a bcrypt hash of the password
func (a *authManager) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword validates a password against its hash
func (a *authManager) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// IsTokenExpired checks if a token has expired
func (a *authManager) IsTokenExpired(accessToken string) bool {
	claims, err := a.ParseToken(accessToken)
	if err != nil {
		return true
	}

	return claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now())
}

// GetTokenExpiration returns the expiration time of a token
func (a *authManager) GetTokenExpiration(accessToken string) (*time.Time, error) {
	claims, err := a.ParseToken(accessToken)
	if err != nil {
		return nil, err
	}

	if claims.ExpiresAt == nil {
		return nil, fmt.Errorf("token has no expiration")
	}

	return &claims.ExpiresAt.Time, nil
}

// GenerateAccessTokenV2 creates a new device-aware v2 JWT access token.
// It keeps the existing secret and expiry configuration to remain compatible
// with current deployment settings.
func (a *authManager) GenerateAccessTokenV2(userID, deviceID string) (string, error) {
	claims := &TokenClaimsV2{
		Payload: Payload{
			UserId:   userID,
			DeviceId: deviceID,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(a.config.AccessTokenExpiry)),
			Issuer:    "growth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.JwtSecretKey))
}

// ParseTokenV2 tries to parse an access token as v1 first for backward
// compatibility, then as v2 with device information. This mirrors the
// behavior from the horjun project while staying compatible with existing
// tokens.
func (a *authManager) ParseTokenV2(accessToken string) (Payload, error) {
	// Attempt to parse as existing v1 token
	claims, err := a.ParseToken(accessToken)
	if err == nil {
		return Payload{UserId: claims.UserId}, nil
	}

	// Fallback to v2 token format
	var claimsV2 TokenClaimsV2
	_, err = jwt.ParseWithClaims(accessToken, &claimsV2, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JwtSecretKey), nil
	})
	if err != nil {
		logx.Errorf("ParseTokenV2.ParseWithClaims - error: %v", err)
		return Payload{}, err
	}

	return claimsV2.Payload, nil
}

// IsPayloadOfAccessTokenV2 checks whether the given payload belongs to a
// device-aware v2 access token.
func (a *authManager) IsPayloadOfAccessTokenV2(payload Payload) bool {
	return payload.UserId != "" && payload.DeviceId != ""
}
