package mfa

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

func TestEncryptDecryptSecret_RoundTrip(t *testing.T) {
	key := testKey(t)
	enc, err := EncryptSecret(key, "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	require.NotContains(t, enc, "JBSWY3DPEHPK3PXP")

	dec, err := DecryptSecret(key, enc)
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", dec)
}

func TestDecryptSecret_WrongKey(t *testing.T) {
	enc, err := EncryptSecret(testKey(t), "JBSWY3DPEHPK3PXP")
	require.NoError(t, err)
	_, err = DecryptSecret(testKey(t), enc)
	require.Error(t, err)
}

func TestParseKey(t *testing.T) {
	_, err := ParseKey("not-base64!")
	require.Error(t, err)
	_, err = ParseKey(base64.StdEncoding.EncodeToString([]byte("too short")))
	require.ErrorIs(t, err, ErrBadKeyLength)
	key, err := ParseKey(base64.StdEncoding.EncodeToString(testKey(t)))
	require.NoError(t, err)
	require.Len(t, key, 32)
}

func TestNewTicket_UniqueAndHashStable(t *testing.T) {
	tok1, h1, err := NewTicket()
	require.NoError(t, err)
	tok2, h2, err := NewTicket()
	require.NoError(t, err)

	require.NotEqual(t, tok1, tok2)
	require.Len(t, tok1, 64) // 32 bytes hex-encoded
	require.Equal(t, h1, HashTicket(tok1))
	require.Equal(t, h2, HashTicket(tok2))
	require.NotEqual(t, h1, h2)
	require.NotContains(t, h1, tok1)
}

func TestValidateCode(t *testing.T) {
	key, err := NewTOTPKey("Growth Admin", "admin@example.com")
	require.NoError(t, err)

	code, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)
	require.True(t, ValidateCode(key.Secret(), code))
	require.False(t, ValidateCode(key.Secret(), "000000"))
}

func TestQRCodeDataURL(t *testing.T) {
	key, err := NewTOTPKey("Growth Admin", "admin@example.com")
	require.NoError(t, err)
	dataURL, err := QRCodeDataURL(key)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(dataURL, "data:image/png;base64,"))
}

func TestNewBackupCodes(t *testing.T) {
	codes, hashes, err := NewBackupCodes()
	require.NoError(t, err)
	require.Len(t, codes, BackupCodeCount)
	require.Len(t, hashes, BackupCodeCount)

	seen := map[string]bool{}
	for _, c := range codes {
		require.Len(t, c, backupCodeLen+1) // 8 chars + dash
		require.NotContains(t, c, "0")
		require.NotContains(t, c, "O")
		require.False(t, seen[c], "duplicate backup code %q", c)
		seen[c] = true
	}
}

func TestHashBackupCode_Normalization(t *testing.T) {
	codes, hashes, err := NewBackupCodes()
	require.NoError(t, err)

	raw := strings.ReplaceAll(codes[0], "-", "")
	require.Equal(t, hashes[0], HashBackupCode(codes[0]))
	require.Equal(t, hashes[0], HashBackupCode(raw))
	require.Equal(t, hashes[0], HashBackupCode(strings.ToLower(raw)))
	require.NotEqual(t, hashes[0], HashBackupCode("AAAAAAAAA"))
}
