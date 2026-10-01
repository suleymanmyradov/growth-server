// Package unsubtoken mints and verifies the HMAC-signed tokens embedded in
// notification emails' List-Unsubscribe URLs (RFC 8058 one-click unsubscribe).
// The signer (delivery worker) and verifier (UnsubscribeEmail RPC) run in the
// same binary; the secret comes from Email.UnsubscribeSecret, falling back to
// ServiceAuth.Secret.
package unsubtoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// purpose is mixed into the HMAC input so unsubscribe tokens can never be
// confused with signatures minted for other uses of the shared secret.
const purpose = "growth:email-unsubscribe:v1:"

// ErrInvalid is returned for malformed or wrongly-signed tokens.
var ErrInvalid = errors.New("unsubtoken: invalid token")

// Sign returns "<userID>.<base64url-hmac>" authenticating userID for email
// unsubscribe. Tokens never expire — CAN-SPAM requires unsubscribe links to
// keep working for at least 30 days and rotating the secret is the only
// revocation path.
func Sign(secret string, userID uuid.UUID) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(purpose))
	mac.Write([]byte(userID.String()))
	return userID.String() + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Verify checks the token's signature and returns the authenticated user ID.
func Verify(secret, token string) (uuid.UUID, error) {
	raw, _, ok := strings.Cut(token, ".")
	if !ok {
		return uuid.Nil, ErrInvalid
	}
	userID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ErrInvalid
	}
	if !hmac.Equal([]byte(token), []byte(Sign(secret, userID))) {
		return uuid.Nil, ErrInvalid
	}
	return userID, nil
}
