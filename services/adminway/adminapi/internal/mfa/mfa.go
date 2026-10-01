// Package mfa holds adminway's TOTP helpers: AES-GCM encryption for secrets
// at rest, pre-auth ticket generation/hashing, TOTP generation/validation,
// QR rendering, and one-time backup codes. DB access stays in internal/repository.
package mfa

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Ticket purposes (admin_mfa_tickets.purpose CHECK constraint).
const (
	TicketPurposeVerify = "verify" // password proven; TOTP still owed
	TicketPurposeEnroll = "enroll" // password proven; TOTP enrollment still owed
)

const (
	// ticketBytes is a 256-bit client-facing token. Only its SHA-256 hash is
	// stored, so a DB read cannot replay a live ticket.
	ticketBytes = 32
	// BackupCodeCount codes are issued per enrollment.
	BackupCodeCount = 10
	// backupCodeLen chars from backupCodeAlphabet (~40 bits per code) — short
	// enough to type, still brute-force-bound by ticket attempt limits.
	backupCodeLen      = 8
	backupCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I
)

var (
	ErrBadKeyLength  = errors.New("mfa encryption key must be 32 bytes")
	ErrDecrypt       = errors.New("mfa secret decryption failed")
	ErrCiphertextBad = errors.New("mfa ciphertext malformed")
)

// ParseKey decodes the base64-encoded 32-byte AES-256 key from config.
func ParseKey(keyB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode mfa key: %w", err)
	}
	if len(key) != 32 {
		return nil, ErrBadKeyLength
	}
	return key, nil
}

// EncryptSecret AES-256-GCM-encrypts a TOTP secret; output is base64
// (nonce prepended). Decrypt with DecryptSecret.
func EncryptSecret(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// DecryptSecret reverses EncryptSecret.
func DecryptSecret(key []byte, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrCiphertextBad
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", ErrCiphertextBad
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(plain), nil
}

// NewTicket returns the client-facing token (hex) and the hash to store.
func NewTicket() (token, tokenHash string, err error) {
	buf := make([]byte, ticketBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(buf)
	return token, HashTicket(token), nil
}

// HashTicket is the stored form of a client ticket token.
func HashTicket(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewTOTPKey generates a new TOTP secret for the given account.
func NewTOTPKey(issuer, accountName string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
}

// ValidateCode checks a 6-digit TOTP against the plaintext base32 secret.
// pquerna's totp.Validate uses the standard 30s period with ±1 step skew.
func ValidateCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// QRCodeDataURL renders the key's otpauth URI as a PNG data URL so the
// frontend needs no QR dependency.
func QRCodeDataURL(key *otp.Key) (string, error) {
	img, err := key.Image(256, 256)
	if err != nil {
		return "", fmt.Errorf("render qr: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode qr png: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// NewBackupCodes returns the plaintext codes (shown once) and the hashes to
// store. Codes look like "K7QX-9ZPM" — group of 4 + 4 for readability.
func NewBackupCodes() (codes []string, hashes []string, err error) {
	codes = make([]string, 0, BackupCodeCount)
	hashes = make([]string, 0, BackupCodeCount)
	for range BackupCodeCount {
		code, err := randomCode()
		if err != nil {
			return nil, nil, err
		}
		codes = append(codes, code)
		hashes = append(hashes, HashBackupCode(code))
	}
	return codes, hashes, nil
}

// HashBackupCode normalizes (strips '-', uppercases) then SHA-256 hashes.
func HashBackupCode(code string) string {
	norm := normalizeCode(code)
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

func randomCode() (string, error) {
	buf := make([]byte, backupCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 0, backupCodeLen+1)
	for i, b := range buf {
		if i == backupCodeLen/2 {
			out = append(out, '-')
		}
		out = append(out, backupCodeAlphabet[int(b)%len(backupCodeAlphabet)])
	}
	return string(out), nil
}

func normalizeCode(code string) string {
	out := make([]byte, 0, len(code))
	for _, c := range []byte(code) {
		switch {
		case c == '-' || c == ' ':
			continue
		case c >= 'a' && c <= 'z':
			out = append(out, c-'a'+'A')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
