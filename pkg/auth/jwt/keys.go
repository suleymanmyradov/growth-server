package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
)

// Key-pair handling for asymmetric token signing (ES256, ECDSA P-256).
//
// PEM material is commonly passed through environment variables, where it is
// stored on a single line with literal "\n" escapes. normalizePEM accepts both
// that form and real multi-line PEM.

// GenerateKeyPair returns a fresh ECDSA P-256 private/public key pair encoded
// as PEM (PKCS#8 "PRIVATE KEY" and SPKI "PUBLIC KEY"). Intended for tooling
// (scripts/gen-jwt-keys.sh) and tests.
func GenerateKeyPair() (privatePEM, publicPEM string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal public key: %w", err)
	}

	privatePEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))
	publicPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return privatePEM, publicPEM, nil
}

// ParsePrivateKeyPEM parses a PEM-encoded ECDSA private key. Accepts PKCS#8
// ("PRIVATE KEY") and SEC1 ("EC PRIVATE KEY") blocks.
func ParsePrivateKeyPEM(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemStr)))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		ecKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is %T, expected *ecdsa.PrivateKey", key)
		}
		return ecKey, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse EC private key: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q for private key", block.Type)
	}
}

// ParsePublicKeyPEM parses a PEM-encoded ECDSA public key (SPKI "PUBLIC KEY").
func ParsePublicKeyPEM(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemStr)))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is %T, expected *ecdsa.PublicKey", key)
	}
	return ecKey, nil
}

// KeyID returns a stable key ID (kid) for a public key: the first 16 hex
// chars of the SHA-256 digest of its SPKI DER encoding. Signers embed it in
// the JWT header so verifiers with multiple trusted keys (future JWKS) can
// pick the right one; single-key verifiers ignore it.
func KeyID(pub *ecdsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:8])
}

// normalizePEM converts a single-line env-style PEM (literal "\n" escapes)
// into real PEM. Values that already contain newlines pass through unchanged.
func normalizePEM(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), `\n`, "\n")
}
