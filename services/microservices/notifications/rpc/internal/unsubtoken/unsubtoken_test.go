package unsubtoken

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignVerify_RoundTrip(t *testing.T) {
	userID := uuid.New()
	token := Sign("test-secret-at-least-32-bytes-long", userID)

	got, err := Verify("test-secret-at-least-32-bytes-long", token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestVerify_WrongSecret(t *testing.T) {
	token := Sign("secret-a", uuid.New())

	_, err := Verify("secret-b", token)
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestVerify_TamperedUserID(t *testing.T) {
	token := Sign("secret", uuid.New())
	// Swap in a different user ID, keep the original signature.
	_, sig, _ := strings.Cut(token, ".")
	forged := uuid.New().String() + "." + sig

	_, err := Verify("secret", forged)
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestVerify_Malformed(t *testing.T) {
	for _, token := range []string{"", "no-dot", ".sig", "not-a-uuid.abc", "uid."} {
		_, err := Verify("secret", token)
		assert.ErrorIs(t, err, ErrInvalid, "token %q", token)
	}
}
