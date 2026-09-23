package paddle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signHeader builds a Paddle-Signature header value for the given ts/body,
// mimicking how Paddle signs: hex(HMAC-SHA256(secret, "ts:body")).
func signHeader(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write([]byte(":"))
	_, _ = mac.Write(body)
	return fmt.Sprintf("ts=%s;h1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestVerifySignature_Valid(t *testing.T) {
	secret := "pdl_ntfset_01jtestsecret"
	body := []byte(`{"event_id":"evt_01","event_type":"transaction.completed","data":{}}`)
	header := signHeader(secret, fmt.Sprintf("%d", time.Now().Unix()), body)

	require.NoError(t, VerifySignature(body, header, secret))
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	body := []byte(`{"event_id":"evt_02"}`)
	header := signHeader("pdl_ntfset_wrong", fmt.Sprintf("%d", time.Now().Unix()), body)

	err := VerifySignature(body, header, "pdl_ntfset_right")
	assert.ErrorIs(t, err, ErrSignatureMismatch)
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	secret := "pdl_ntfset_01jtestsecret"
	body := []byte(`{"event_id":"evt_03"}`)
	header := signHeader(secret, fmt.Sprintf("%d", time.Now().Unix()), body)

	err := VerifySignature([]byte(`{"event_id":"evt_HACKED"}`), header, secret)
	assert.ErrorIs(t, err, ErrSignatureMismatch)
}

func TestVerifySignature_StaleTimestamp(t *testing.T) {
	secret := "pdl_ntfset_01jtestsecret"
	body := []byte(`{"event_id":"evt_04"}`)

	// Signed correctly but 10 minutes ago — outside the 5-minute tolerance.
	oldTS := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	err := VerifySignature(body, signHeader(secret, oldTS, body), secret)
	assert.ErrorIs(t, err, ErrStaleTimestamp)

	// A timestamp in the future is rejected too — bounds replay both ways.
	futureTS := fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix())
	err = VerifySignature(body, signHeader(secret, futureTS, body), secret)
	assert.ErrorIs(t, err, ErrStaleTimestamp)
}

func TestVerifySignature_MultipleH1(t *testing.T) {
	// During secret rotation Paddle sends several h1 values; any match wins.
	secret := "pdl_ntfset_newsecret"
	body := []byte(`{"event_id":"evt_05"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())

	oldMAC := hmac.New(sha256.New, []byte("pdl_ntfset_oldsecret"))
	_, _ = oldMAC.Write([]byte(ts + ":" + string(body)))
	oldSig := hex.EncodeToString(oldMAC.Sum(nil))

	newMAC := hmac.New(sha256.New, []byte(secret))
	_, _ = newMAC.Write([]byte(ts + ":" + string(body)))
	newSig := hex.EncodeToString(newMAC.Sum(nil))

	header := fmt.Sprintf("ts=%s;h1=%s;h1=%s", ts, oldSig, newSig)
	require.NoError(t, VerifySignature(body, header, secret))

	// If none of the rotated signatures match our secret, reject.
	header = fmt.Sprintf("ts=%s;h1=%s", ts, oldSig)
	err := VerifySignature(body, header, secret)
	assert.ErrorIs(t, err, ErrSignatureMismatch)
}

func TestVerifySignature_MalformedHeader(t *testing.T) {
	secret := "pdl_ntfset_01jtestsecret"
	body := []byte(`{"event_id":"evt_06"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	goodSig := signHeader(secret, ts, body)[len("ts="+ts+";h1="):]

	cases := map[string]string{
		"empty":            "",
		"garbage":          "not-a-signature",
		"no ts":            "h1=" + goodSig,
		"no h1":            "ts=" + ts,
		"empty h1":         "ts=" + ts + ";h1=",
		"non-numeric ts":   "ts=abc;h1=" + goodSig,
		"semicolons only":  ";;;",
		"ts only pair set": "h1;ts=" + ts,
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			err := VerifySignature(body, header, secret)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrMalformedSignature) || errors.Is(err, ErrSignatureMismatch),
				"expected malformed or mismatch, got %v", err)
		})
	}
}

func TestVerifySignature_NonHexH1(t *testing.T) {
	secret := "pdl_ntfset_01jtestsecret"
	body := []byte(`{"event_id":"evt_07"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())

	// A non-hex h1 is skipped rather than aborting verification of other h1s.
	err := VerifySignature(body, "ts="+ts+";h1=zzzznothex", secret)
	assert.ErrorIs(t, err, ErrSignatureMismatch)

	valid := signHeader(secret, ts, body)
	header := "ts=" + ts + ";h1=zzzznothex;h1=" + valid[len("ts="+ts+";h1="):]
	require.NoError(t, VerifySignature(body, header, secret))
}

func TestVerifySignature_EmptySecret(t *testing.T) {
	body := []byte(`{}`)
	header := signHeader("whatever", fmt.Sprintf("%d", time.Now().Unix()), body)
	err := VerifySignature(body, header, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
}

func TestVerifier_CustomTolerance(t *testing.T) {
	secret := "pdl_ntfset_01jtestsecret"
	body := []byte(`{"event_id":"evt_08"}`)

	// Tight tolerance matching Paddle SDK's own 5s default.
	v := NewVerifier(secret, WithTimestampTolerance(5*time.Second))

	recentTS := fmt.Sprintf("%d", time.Now().Add(-3*time.Second).Unix())
	require.NoError(t, v.Verify(body, signHeader(secret, recentTS, body)))

	staleTS := fmt.Sprintf("%d", time.Now().Add(-30*time.Second).Unix())
	err := v.Verify(body, signHeader(secret, staleTS, body))
	assert.ErrorIs(t, err, ErrStaleTimestamp)
}

func TestVerifySignature_TimestampPartOfSignedPayload(t *testing.T) {
	// The ts is part of the signed material — a header with a valid h1 for a
	// different ts must not verify.
	secret := "pdl_ntfset_01jtestsecret"
	body := []byte(`{"event_id":"evt_09"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())

	otherTS := fmt.Sprintf("%d", time.Now().Add(-time.Minute).Unix())
	sigForOtherTS := signHeader(secret, otherTS, body)[len("ts="+otherTS+";h1="):]

	err := VerifySignature(body, "ts="+ts+";h1="+sigForOtherTS, secret)
	assert.ErrorIs(t, err, ErrSignatureMismatch)
}
