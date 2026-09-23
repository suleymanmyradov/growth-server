package paddle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SignatureHeader is the HTTP header Paddle sets on every webhook delivery.
const SignatureHeader = "Paddle-Signature"

// DefaultTimestampTolerance is the maximum allowed drift between the ts in
// the signature header and the local clock before a delivery is rejected as a
// possible replay. Paddle's own SDKs default to five seconds; we allow five
// minutes to tolerate clock skew on our servers — tighten it with
// WithTimestampTolerance.
const DefaultTimestampTolerance = 5 * time.Minute

// Verification failures, distinguishable with errors.Is so handlers can map
// them to the right response (400 for malformed, 401 for mismatch/stale).
var (
	// ErrMalformedSignature means the header could not be parsed into
	// ts=...;h1=... parts.
	ErrMalformedSignature = errors.New("paddle: malformed signature header")
	// ErrStaleTimestamp means ts is outside the allowed tolerance — the
	// delivery is a replay or the clocks disagree.
	ErrStaleTimestamp = errors.New("paddle: signature timestamp outside tolerance")
	// ErrSignatureMismatch means no h1 in the header matched the expected
	// HMAC — wrong secret or tampered body.
	ErrSignatureMismatch = errors.New("paddle: signature mismatch")
)

// Verifier checks Paddle-Signature webhook headers. Construct with
// NewVerifier; for a one-off check with defaults use VerifySignature.
type Verifier struct {
	secret    string
	tolerance time.Duration
	now       func() time.Time
}

// VerifierOption configures a Verifier.
type VerifierOption func(*Verifier)

// WithTimestampTolerance overrides the maximum allowed drift between the
// signature timestamp and the local clock.
func WithTimestampTolerance(d time.Duration) VerifierOption {
	return func(v *Verifier) { v.tolerance = d }
}

// NewVerifier returns a Verifier for the webhook endpoint secret shown in the
// Paddle dashboard (pdl_ntfset_...).
func NewVerifier(secret string, opts ...VerifierOption) *Verifier {
	v := &Verifier{secret: secret, tolerance: DefaultTimestampTolerance, now: time.Now}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// VerifySignature verifies payload (the exact raw request body) against a
// Paddle-Signature header value using the default timestamp tolerance.
// Returns nil when the signature is valid.
func VerifySignature(payload []byte, signatureHeader, secret string) error {
	return NewVerifier(secret).Verify(payload, signatureHeader)
}

// Verify verifies payload (the exact raw request body) against a
// Paddle-Signature header value. Returns nil when valid, else one of
// ErrMalformedSignature, ErrStaleTimestamp, or ErrSignatureMismatch.
//
// The header has the form "ts=<unix>;h1=<hex hmac-sha256>" and the signed
// content is "ts:body". Multiple h1 values may be present while Paddle
// rotates secrets — verification succeeds if any one matches.
func (v *Verifier) Verify(payload []byte, signatureHeader string) error {
	if v.secret == "" {
		return fmt.Errorf("paddle: webhook secret is not configured")
	}

	ts, signatures, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		return err
	}

	tsUnix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: ts %q is not a unix timestamp", ErrMalformedSignature, ts)
	}
	if drift := v.now().Sub(time.Unix(tsUnix, 0)); drift > v.tolerance || drift < -v.tolerance {
		return fmt.Errorf("%w: ts=%d drift=%s", ErrStaleTimestamp, tsUnix, drift.Truncate(time.Second))
	}

	// Paddle signs the literal "ts:body" — use the header's raw ts string,
	// not the parsed integer, so non-canonical ts values still verify the
	// way Paddle computed them.
	mac := hmac.New(sha256.New, []byte(v.secret))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write([]byte(":"))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)

	for _, sig := range signatures {
		decoded, err := hex.DecodeString(sig)
		if err != nil {
			continue // not hex — can't match, but another h1 might
		}
		if hmac.Equal(decoded, expected) {
			return nil
		}
	}
	return ErrSignatureMismatch
}

// parseSignatureHeader splits "ts=...;h1=...;h1=..." into the raw timestamp
// string and the list of h1 signature hex values.
func parseSignatureHeader(header string) (ts string, signatures []string, err error) {
	for _, part := range strings.Split(header, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "ts":
			ts = kv[1]
		case "h1":
			signatures = append(signatures, kv[1])
		}
		// Unknown keys are ignored for forward compatibility.
	}
	if ts == "" {
		return "", nil, fmt.Errorf("%w: missing ts", ErrMalformedSignature)
	}
	if len(signatures) == 0 {
		return "", nil, fmt.Errorf("%w: missing h1", ErrMalformedSignature)
	}
	return ts, signatures, nil
}
