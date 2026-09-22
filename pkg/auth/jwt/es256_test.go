package jwt

import (
	"context"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// testKeyPair generates a fresh P-256 pair per test and returns PEM strings.
func testKeyPair(t *testing.T) (privatePEM, publicPEM string) {
	t.Helper()
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	return priv, pub
}

func testECConfig(privatePEM string) Config {
	return Config{
		PrivateKey:            privatePEM,
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  time.Minute,
		RefreshExpiryDuration: time.Hour,
	}
}

func TestES256_SignAndVerify(t *testing.T) {
	privPEM, pubPEM := testKeyPair(t)
	maker, err := NewTokenMaker(testECConfig(privPEM), nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	resp, err := maker.CreateAccessToken(context.Background(), uuid.New(), "test-user", []string{"user"}, uuid.New())
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	// The token must be ES256 and carry the kid header.
	parsed, _, err := jwt.NewParser().ParseUnverified(resp.Token, &TokenClaims{})
	if err != nil {
		t.Fatalf("parse unverified: %v", err)
	}
	if parsed.Method.Alg() != "ES256" {
		t.Fatalf("expected alg ES256, got %s", parsed.Method.Alg())
	}
	kid, _ := parsed.Header["kid"].(string)
	pub, _ := ParsePublicKeyPEM(pubPEM)
	if kid != KeyID(pub) {
		t.Fatalf("expected kid %q, got %q", KeyID(pub), kid)
	}

	// A verify-only Verifier holding just the public key must accept it.
	verifier, err := NewVerifier(Config{
		PublicKey: pubPEM,
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	claims, err := verifier.VerifyAccessToken(context.Background(), resp.Token)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if claims.Username != "test-user" {
		t.Fatalf("expected username test-user, got %q", claims.Username)
	}
}

func TestES256_VerifierRejectsWrongKey(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	_, otherPubPEM := testKeyPair(t)

	maker, err := NewTokenMaker(testECConfig(privPEM), nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}
	resp, err := maker.CreateAccessToken(context.Background(), uuid.New(), "u", []string{"user"}, uuid.New())
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	verifier, err := NewVerifier(Config{PublicKey: otherPubPEM, Issuer: "test-issuer", Audience: "test-audience"})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	if _, err := verifier.VerifyAccessToken(context.Background(), resp.Token); err == nil {
		t.Fatal("expected token signed by a different key to be rejected")
	}
}

func TestES256_RefreshTokenAndRevocation(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	repo := newMockRevocationRepo()
	maker, err := NewTokenMaker(testECConfig(privPEM), repo)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	refresh, err := maker.CreateRefreshToken(context.Background(), uuid.New(), "u", []string{"user"}, uuid.New())
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
	if _, err := maker.VerifyRefreshToken(context.Background(), refresh.Token); err != nil {
		t.Fatalf("verify refresh token: %v", err)
	}
	if err := maker.RevokeRefreshToken(context.Background(), refresh.Token); err != nil {
		t.Fatalf("revoke refresh token: %v", err)
	}
	if _, err := maker.VerifyRefreshToken(context.Background(), refresh.Token); err == nil {
		t.Fatal("expected revoked refresh token to be rejected")
	}
}

func TestDualVerify_LegacyHS256AcceptedDuringWindow(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	cfg := testECConfig(privPEM)
	cfg.Secret = "legacy-secret-must-be-at-least-32-bytes"

	maker, err := NewTokenMaker(cfg, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}

	// A pre-cutover HS256 token must still verify while Secret is configured.
	legacyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &TokenClaims{
		ID:        uuid.New(),
		Subject:   uuid.New(),
		SessionID: uuid.New(),
		Username:  "legacy-user",
		Roles:     []string{"user"},
		Issuer:    "test-issuer",
		Audience:  []string{"test-audience"},
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		TokenType: AccessToken,
	}).SignedString([]byte(cfg.Secret))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}

	verifier, err := NewVerifier(Config{
		PublicKey: mustPublicFromPrivate(t, privPEM),
		Secret:    cfg.Secret,
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	if _, err := verifier.VerifyAccessToken(context.Background(), legacyToken); err != nil {
		t.Fatalf("expected legacy HS256 token to verify during window: %v", err)
	}
	if _, err := maker.VerifyAccessToken(context.Background(), legacyToken); err != nil {
		t.Fatalf("expected maker to accept legacy HS256 token during window: %v", err)
	}
}

func TestES256_HS256RejectedAfterWindow(t *testing.T) {
	privPEM, pubPEM := testKeyPair(t)
	maker, err := NewTokenMaker(testECConfig(privPEM), nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}
	verifier, err := NewVerifier(Config{PublicKey: pubPEM, Issuer: "test-issuer", Audience: "test-audience"})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	// With no Secret configured, HS256 tokens must be rejected — including a
	// token signed with the public key PEM as the HMAC secret (the classic
	// RS256→HS256 algorithm-confusion attack).
	confused, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &TokenClaims{
		ID:        uuid.New(),
		Subject:   uuid.New(),
		SessionID: uuid.New(),
		Issuer:    "test-issuer",
		Audience:  []string{"test-audience"},
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		TokenType: AccessToken,
	}).SignedString([]byte(pubPEM))
	if err != nil {
		t.Fatalf("sign confused token: %v", err)
	}
	if _, err := verifier.VerifyAccessToken(context.Background(), confused); err == nil {
		t.Fatal("expected HS256 token signed with public key PEM to be rejected")
	}
	if _, err := maker.VerifyAccessToken(context.Background(), confused); err == nil {
		t.Fatal("expected maker to reject HS256 token signed with public key PEM")
	}
}

func TestTokenMaker_PublicKeyOnlyCannotSign(t *testing.T) {
	_, pubPEM := testKeyPair(t)
	maker, err := NewTokenMaker(Config{
		PublicKey: pubPEM,
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	}, nil)
	if err != nil {
		t.Fatalf("create token maker: %v", err)
	}
	if _, err := maker.CreateAccessToken(context.Background(), uuid.New(), "u", nil, uuid.New()); err == nil {
		t.Fatal("expected Create* to fail without a signing credential")
	}
}

func TestParseKeys_EnvarEscapedNewlines(t *testing.T) {
	privPEM, pubPEM := testKeyPair(t)

	// Env vars carry PEMs on one line with literal \n escapes.
	esc := func(s string) string { return strings.ReplaceAll(s, "\n", `\n`) }

	if _, err := ParsePrivateKeyPEM(esc(privPEM)); err != nil {
		t.Fatalf("parse escaped private key: %v", err)
	}
	if _, err := ParsePublicKeyPEM(esc(pubPEM)); err != nil {
		t.Fatalf("parse escaped public key: %v", err)
	}

	maker, err := NewTokenMaker(testECConfig(esc(privPEM)), nil)
	if err != nil {
		t.Fatalf("create token maker from escaped PEM: %v", err)
	}
	resp, err := maker.CreateAccessToken(context.Background(), uuid.New(), "u", nil, uuid.New())
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}
	verifier, err := NewVerifier(Config{PublicKey: esc(pubPEM), Issuer: "test-issuer", Audience: "test-audience"})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	if _, err := verifier.VerifyAccessToken(context.Background(), resp.Token); err != nil {
		t.Fatalf("verify with escaped public key: %v", err)
	}
}

func TestNewTokenMaker_RequiresCredential(t *testing.T) {
	if _, err := NewTokenMaker(Config{Issuer: "i", Audience: "a"}, nil); err == nil {
		t.Fatal("expected error when no credential is configured")
	}
}

func TestNewVerifier_RequiresKeyOrSecret(t *testing.T) {
	if _, err := NewVerifier(Config{Issuer: "i", Audience: "a"}); err == nil {
		t.Fatal("expected error when neither PublicKey nor Secret is set")
	}
}

// mustPublicFromPrivate derives the PEM public key from a PEM private key.
func mustPublicFromPrivate(t *testing.T, privPEM string) string {
	t.Helper()
	priv, err := ParsePrivateKeyPEM(privPEM)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// Sanity: generated keys are real P-256 ECDSA.
func TestGenerateKeyPair_ProducesP256(t *testing.T) {
	privPEM, _ := testKeyPair(t)
	key, err := ParsePrivateKeyPEM(privPEM)
	if err != nil {
		t.Fatalf("parse generated key: %v", err)
	}
	if key.Curve != elliptic.P256() {
		t.Fatalf("expected P-256, got %v", key.Curve.Params().Name)
	}
}
