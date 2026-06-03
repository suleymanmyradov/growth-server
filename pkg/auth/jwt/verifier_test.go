package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewVerifier_Validation(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tests := []struct {
		name    string
		cfg     VerifierConfig
		wantErr bool
	}{
		{
			name: "valid",
			cfg: VerifierConfig{
				Issuer:   "test-issuer",
				Audience: "test-audience",
				KeyFunc:  &StaticKeyFunc{Key: &rsaKey.PublicKey},
			},
			wantErr: false,
		},
		{
			name: "missing issuer",
			cfg: VerifierConfig{
				Audience: "test-audience",
				KeyFunc:  &StaticKeyFunc{Key: &rsaKey.PublicKey},
			},
			wantErr: true,
		},
		{
			name: "missing audience",
			cfg: VerifierConfig{
				Issuer:  "test-issuer",
				KeyFunc: &StaticKeyFunc{Key: &rsaKey.PublicKey},
			},
			wantErr: true,
		},
		{
			name: "missing keyFunc",
			cfg: VerifierConfig{
				Issuer:   "test-issuer",
				Audience: "test-audience",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVerifier(tt.cfg)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRSAVerifier(t *testing.T) {
	require := require.New(t)

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(err)
	publicKey := &privateKey.PublicKey

	issuer := "test-issuer"
	audience := "test-audience"
	userID := uuid.New()
	sessionID := uuid.New()

	// Create a token signed with the private key
	now := time.Now()
	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   userID,
		SessionID: sessionID,
		Username:  "testuser",
		Roles:     []string{"user", "admin"},
		Issuer:    issuer,
		Audience:  []string{audience},
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
		NotBefore: now,
		TokenType: AccessToken,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &claims)
	token.Header["kid"] = "key-1"
	tokenString, err := token.SignedString(privateKey)
	require.NoError(err)

	// Verify with the public key
	verifier, err := NewVerifier(VerifierConfig{
		Issuer:   issuer,
		Audience: audience,
		KeyFunc:  &StaticKeyFunc{Key: publicKey},
	})
	require.NoError(err)

	parsed, err := verifier.VerifyAccessToken(context.Background(), tokenString)
	require.NoError(err)
	require.Equal(userID, parsed.Subject)
	require.Equal("testuser", parsed.Username)
	require.Equal([]string{"user", "admin"}, parsed.Roles)
	require.Equal(sessionID, parsed.SessionID)

	// Wrong public key should fail
	wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	badVerifier, _ := NewVerifier(VerifierConfig{
		Issuer:   issuer,
		Audience: audience,
		KeyFunc:  &StaticKeyFunc{Key: &wrongKey.PublicKey},
	})
	_, err = badVerifier.VerifyAccessToken(context.Background(), tokenString)
	require.ErrorIs(err, ErrInvalidToken)
}

func TestECDSAVerifier(t *testing.T) {
	require := require.New(t)

	// Generate ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(err)
	publicKey := &privateKey.PublicKey

	issuer := "test-issuer"
	audience := "test-audience"
	userID := uuid.New()

	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   userID,
		SessionID: uuid.New(),
		Username:  "ecdsa-user",
		Issuer:    issuer,
		Audience:  []string{audience},
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		NotBefore: time.Now(),
		TokenType: AccessToken,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, &claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(err)

	verifier, err := NewVerifier(VerifierConfig{
		Issuer:   issuer,
		Audience: audience,
		KeyFunc:  &StaticKeyFunc{Key: publicKey},
	})
	require.NoError(err)

	parsed, err := verifier.VerifyAccessToken(context.Background(), tokenString)
	require.NoError(err)
	require.Equal(userID, parsed.Subject)
}

func TestEd25519Verifier(t *testing.T) {
	require := require.New(t)

	// Generate Ed25519 key pair
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(err)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	issuer := "test-issuer"
	audience := "test-audience"
	userID := uuid.New()

	claims := TokenClaims{
		ID:        uuid.New(),
		Subject:   userID,
		SessionID: uuid.New(),
		Username:  "eddsa-user",
		Issuer:    issuer,
		Audience:  []string{audience},
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		NotBefore: time.Now(),
		TokenType: AccessToken,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, &claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(err)

	verifier, err := NewVerifier(VerifierConfig{
		Issuer:   issuer,
		Audience: audience,
		KeyFunc:  &StaticKeyFunc{Key: publicKey},
	})
	require.NoError(err)

	parsed, err := verifier.VerifyAccessToken(context.Background(), tokenString)
	require.NoError(err)
	require.Equal(userID, parsed.Subject)
}

func TestStaticKeyFunc_NilKey(t *testing.T) {
	kf := &StaticKeyFunc{Key: nil}
	_, err := kf.GetKey("", "")
	require.Error(t, err)
}

func TestVerifier_SatisfiesTokenVerifierInterface(t *testing.T) {
	// Compile-time check that *Verifier satisfies the interface expected by mdpropagate
	var _ interface {
		VerifyAccessToken(ctx context.Context, tokenString string) (*TokenClaims, error)
	} = (*Verifier)(nil)
}
