package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	UserId   string `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type TokenPayload struct {
	UserId   string `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Payload represents the core data carried by v2 access tokens.
// It is intentionally minimal and device-aware to support multi-device auth.
type Payload struct {
	UserId   string `json:"userId"`
	DeviceId string `json:"deviceId"`
}

// TokenClaimsV2 describes the JWT claims structure for v2 access tokens.
// It embeds Payload so we can easily extend the payload in the future.
type TokenClaimsV2 struct {
	Payload `json:"payload"`
	jwt.RegisteredClaims
}

type User struct {
	ID        string    `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password_hash"`
	FullName  string    `json:"full_name" db:"full_name"`
	Bio       string    `json:"bio" db:"bio"`
	Location  string    `json:"location" db:"location"`
	Website   string    `json:"website" db:"website"`
	Interests []string  `json:"interests" db:"interests"`
	AvatarURL string    `json:"avatar_url" db:"avatar_url"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type RefreshToken struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type PasswordResetToken struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	Used      bool      `json:"used" db:"used"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
