// Package apple implements Sign in with Apple server-side verification.
//
// The flow:
//  1. The native client (expo-apple-authentication) obtains an authorization code
//     and an identity token (a signed JWT) from Apple.
//  2. The client sends both to POST /api/v1/auth/apple.
//  3. The auth service verifies the identity token's signature against Apple's
//     public keys (https://appleid.apple.com/auth/keys), validates issuer,
//     audience, and expiry, and extracts the stable subject (`sub`), email, and
//     (only on first authorization) the user's name.
//  4. Optionally, the authorization code is exchanged for a long-lived refresh
//     token using a client-secret JWT signed with the Apple private key (.p8).
//     This is needed only for server-side token revocation / subscription status
//     checks; it is not required to authenticate the user.
//
// Security notes:
//   - The identity token is verified by signature against Apple's JWKS, not by
//     decoding claims alone. This is required because the token is delivered via
//     the client (not over a direct TLS connection to Apple), unlike the Google
//     flow where the code is exchanged directly with Google's token endpoint.
//   - The audience must be the Growth Apple Services ID (the same identifier used
//     for Sign in with Apple in the Apple Developer portal).
//   - Apple only provides the user's real name on the FIRST authorization; the
//     name is delivered in the authorization response, not in the ID token. The
//     client forwards it as AppleFullName.
//   - Apple may return a private relay email (@privaterelay.appleid.com) instead
//     of the user's real email. These are treated as ordinary emails.
package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// appleIssuer is the required `iss` claim for Apple ID tokens.
	appleIssuer = "https://appleid.apple.com"
	// jwksURL is Apple's published public key set for ID token verification.
	jwksURL = "https://appleid.apple.com/auth/keys"
	// tokenURL is Apple's token endpoint for authorization code exchange.
	tokenURL = "https://appleid.apple.com/auth/token"
)

// UserInfo is the subset of Apple profile claims needed for account linking.
type UserInfo struct {
	// Subject is Apple's stable, app-scoped user identifier (`sub`). It is
	// unique per Services ID and stable across authorizations for the same
	// user + Services ID pair.
	Subject string `json:"sub"`
	// Email is the email Apple returns. May be the user's real email or a
	// private relay email (@privaterelay.appleid.com).
	Email string `json:"email"`
	// EmailVerified mirrors the `email_verified` claim. Apple encodes it as
	// a string ("true"/"false") rather than a boolean; this is normalized.
	EmailVerified bool `json:"email_verified"`
	// Name is the user's display name, built from givenName + familyName.
	// Apple only provides this on the FIRST authorization, delivered outside
	// the ID token (in the authorization response). The client forwards it.
	// Empty for subsequent authorizations.
	Name string `json:"name"`
	// GivenName is the user's given name (first name). Empty after first auth.
	GivenName string `json:"given_name"`
	// FamilyName is the user's family name (last name). Empty after first auth.
	FamilyName string `json:"family_name"`
	// IsPrivateRelayEmail is true when Email is an @privaterelay.appleid.com
	// address. These are real, deliverable addresses owned by the user.
	IsPrivateRelayEmail bool `json:"is_private_relay_email"`
}

// Config holds the Apple Sign in with Apple server configuration.
type Config struct {
	// ServiceID is the Apple Services ID used for Sign in with Apple (the
	// audience of the ID token). Distinct from the native app bundle ID when
	// using a web-style Services ID, but may equal the bundle ID for native.
	ServiceID string
	// TeamID is the Apple Developer Team ID (10 characters).
	TeamID string
	// KeyID is the ID of the private key (.p8) created in the Apple Developer
	// portal. Required only for authorization code exchange / token revocation.
	KeyID string
	// PrivateKeyPath is the filesystem path to the .p8 private key. Mutually
	// exclusive with PrivateKey; one must be set when code exchange is used.
	PrivateKeyPath string
	// PrivateKey is the inline PEM-encoded .p8 private key (ECDSA P-256).
	// Mutually exclusive with PrivateKeyPath.
	PrivateKey string
	// RedirectURI is the redirect URI registered with Apple for code exchange.
	// Not required for ID token verification.
	RedirectURI string
	// AllowedRedirectURIs is an explicit allowlist of redirect URIs that
	// clients may supply when exchanging an authorization code. If empty,
	// only the configured RedirectURI is accepted. Mirrors the Google flow's
	// allowlist to prevent authorization code interception.
	AllowedRedirectURIs []string
}

// Verifier verifies Apple ID tokens. It caches Apple's JWKS public keys in
// memory with a TTL so repeated verifications do not re-fetch.
type Verifier struct {
	config     Config
	httpClient *http.Client
	jwksURL    string

	// cachedKeys is the parsed JWKS, cached until jwksExpiry.
	cachedKeys map[string]*rsa.PublicKey
	jwksExpiry time.Time
}

// NewVerifier returns a Verifier for the given config. The HTTP client may be
// nil (http.DefaultClient is used). Injecting a client allows tests to serve a
// mock JWKS endpoint.
func NewVerifier(cfg Config, httpClient *http.Client) *Verifier {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Verifier{config: cfg, httpClient: httpClient, jwksURL: jwksURL}
}

// appleIDTokenClaims is the claim set carried by an Apple ID token. The
// standard registered claims (iss, aud, exp, sub, iat, nonce) are validated by
// the JWT library via the embedded jwt.RegisteredClaims; the Apple-specific
// fields are parsed alongside.
type appleIDTokenClaims struct {
	jwt.RegisteredClaims
	Email          string `json:"email"`
	EmailVerified  any    `json:"email_verified"` // Apple encodes as string or bool
	IsPrivateEmail any    `json:"is_private_email"`
	NonceSupported any    `json:"nonce_supported"`
	// Nonce is present only when the client sent a nonce during authorization.
	Nonce string `json:"nonce,omitempty"`
}

// VerifyIDToken verifies the Apple ID token signature against Apple's public
// keys and validates issuer, audience, and expiry. If nonce is non-empty, the
// token's nonce claim must match. Returns the verified user info.
//
// The audience is validated against Config.ServiceID. The issuer is validated
// against appleIssuer. Expiry is enforced by the JWT library.
func (v *Verifier) VerifyIDToken(ctx context.Context, idToken, nonce string) (UserInfo, error) {
	if idToken == "" {
		return UserInfo{}, fmt.Errorf("apple: id token is required")
	}
	if v.config.ServiceID == "" {
		return UserInfo{}, fmt.Errorf("apple: ServiceID is not configured")
	}

	keys, err := v.fetchJWKS(ctx)
	if err != nil {
		return UserInfo{}, fmt.Errorf("apple: fetch jwks: %w", err)
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.config.ServiceID),
		jwt.WithExpirationRequired(),
	)

	var claims appleIDTokenClaims
	parsed, err := parser.ParseWithClaims(idToken, &claims, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("apple: id token missing kid header")
		}
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("apple: no matching key for kid %s", kid)
		}
		return key, nil
	})
	if err != nil {
		return UserInfo{}, fmt.Errorf("apple: verify id token: %w", err)
	}
	if !parsed.Valid {
		return UserInfo{}, fmt.Errorf("apple: id token is not valid")
	}

	// Optional nonce check (Apple includes `nonce` only when the client sent one
	// during authorization). When the caller provides a nonce, require a match.
	if nonce != "" {
		if claims.Nonce == "" || claims.Nonce != nonce {
			return UserInfo{}, fmt.Errorf("apple: nonce mismatch")
		}
	}

	if claims.Subject == "" {
		return UserInfo{}, fmt.Errorf("apple: id token missing sub claim")
	}

	ui := UserInfo{
		Subject:    claims.Subject,
		Email:      claims.Email,
		GivenName:  "", // populated by the caller from AppleFullName
		FamilyName: "",
	}
	switch ev := claims.EmailVerified.(type) {
	case bool:
		ui.EmailVerified = ev
	case string:
		ui.EmailVerified = strings.EqualFold(ev, "true")
	}
	switch ip := claims.IsPrivateEmail.(type) {
	case bool:
		ui.IsPrivateRelayEmail = ip
	case string:
		ui.IsPrivateRelayEmail = strings.EqualFold(ip, "true")
	}
	if ui.Email == "" {
		// Apple may omit email for subsequent authorizations. That is a hard
		// error for first-time linking; the caller decides how to handle it.
		return UserInfo{}, fmt.Errorf("apple: id token missing email claim")
	}
	return ui, nil
}

// fetchJWKS fetches and parses Apple's JWKS, caching the parsed RSA public keys
// keyed by kid until jwksExpiry.
func (v *Verifier) fetchJWKS(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	if v.cachedKeys != nil && time.Now().Before(v.jwksExpiry) {
		return v.cachedKeys, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("apple: jwks status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("apple: decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		pub, err := jwkToRSAPublicKey(k.N, k.E)
		if err != nil {
			return nil, fmt.Errorf("apple: parse jwk kid=%s: %w", k.Kid, err)
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("apple: jwks contained no usable RSA keys")
	}

	v.cachedKeys = keys
	v.jwksExpiry = time.Now().Add(15 * time.Minute)
	return keys, nil
}

// jwkToRSAPublicKey builds an *rsa.PublicKey from the base64url-encoded JWK
// modulus (n) and exponent (e).
func jwkToRSAPublicKey(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	exponent := 0
	for _, b := range eBytes {
		exponent = exponent<<8 + int(b&0xFF)
	}
	if exponent < 1 {
		return nil, fmt.Errorf("invalid exponent")
	}
	modulus := new(big.Int).SetBytes(nBytes)
	return &rsa.PublicKey{N: modulus, E: exponent}, nil
}

// ExchangeCode exchanges an Apple authorization code for a refresh token.
// This requires the Apple private key (.p8) to sign a client-secret JWT (ES256).
// Returns the refresh token (Apple returns it as `refresh_token`).
//
// This is optional for authentication — the ID token is sufficient to identify
// the user. Code exchange is needed only for server-side token revocation or
// subscription status checks.
func (v *Verifier) ExchangeCode(ctx context.Context, code, redirectURI string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("apple: authorization code is required")
	}
	if v.config.TeamID == "" || v.config.KeyID == "" || v.config.ServiceID == "" {
		return "", fmt.Errorf("apple: TeamID, KeyID, and ServiceID are required for code exchange")
	}
	privKey, err := v.loadPrivateKey()
	if err != nil {
		return "", err
	}

	if redirectURI == "" {
		redirectURI = v.config.RedirectURI
	} else if !isAllowedRedirectURI(redirectURI, v.config.AllowedRedirectURIs, v.config.RedirectURI) {
		return "", fmt.Errorf("apple: redirect URI not allowed")
	}
	if redirectURI == "" {
		return "", fmt.Errorf("apple: redirect URI is required for code exchange")
	}

	clientSecret, err := v.buildClientSecretJWT(privKey, time.Now(), 30*24*time.Hour)
	if err != nil {
		return "", err
	}

	form := url.Values{
		"client_id":     {v.config.ServiceID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("apple: token exchange status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("apple: decode token response: %w", err)
	}
	if tok.RefreshToken == "" {
		return "", fmt.Errorf("apple: token response missing refresh_token")
	}
	return tok.RefreshToken, nil
}

// buildClientSecretJWT builds the Apple client-secret JWT signed with ES256
// using the .p8 private key. The JWT is valid for `validity` from `issuedAt`.
// Apple requires iss=TeamID, sub=ServiceID, aud=appleIssuer.
func (v *Verifier) buildClientSecretJWT(privKey *ecdsa.PrivateKey, issuedAt time.Time, validity time.Duration) (string, error) {
	now := issuedAt
	claims := jwt.RegisteredClaims{
		Issuer:    v.config.TeamID,
		Subject:   v.config.ServiceID,
		Audience:  []string{appleIssuer},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(validity)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = v.config.KeyID
	signed, err := token.SignedString(privKey)
	if err != nil {
		return "", fmt.Errorf("apple: sign client secret: %w", err)
	}
	return signed, nil
}

// loadPrivateKey parses the .p8 ECDSA P-256 private key from PrivateKeyPath or
// PrivateKey.
func (v *Verifier) loadPrivateKey() (*ecdsa.PrivateKey, error) {
	var pemBytes []byte
	switch {
	case v.config.PrivateKey != "":
		pemBytes = []byte(v.config.PrivateKey)
	case v.config.PrivateKeyPath != "":
		b, err := os.ReadFile(v.config.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("apple: read private key: %w", err)
		}
		pemBytes = b
	default:
		return nil, fmt.Errorf("apple: no private key configured (set PrivateKey or PrivateKeyPath)")
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("apple: private key is not valid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Fall back to EC-specific parsing for older PKCS1 EC encodings.
		if ecKey, eerr := x509.ParseECPrivateKey(block.Bytes); eerr == nil {
			return ecKey, nil
		}
		return nil, fmt.Errorf("apple: parse private key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("apple: private key is not ECDSA (got %T)", key)
	}
	return ecKey, nil
}

// IsAllowedRedirectURI checks whether a client-supplied redirect URI is in the
// allowlist. If the allowlist is empty, only the server's configured RedirectURI
// is accepted. Exported so the auth logic can validate the redirect before
// calling ExchangeCode, mirroring the Google flow.
func IsAllowedRedirectURI(uri string, allowed []string, configured string) bool {
	return isAllowedRedirectURI(uri, allowed, configured)
}

func isAllowedRedirectURI(uri string, allowed []string, configured string) bool {
	for _, a := range allowed {
		if a == uri {
			return true
		}
	}
	return uri == configured
}

// FullName builds a display name from given and family name. Returns "" when
// both are empty (the common case for subsequent authorizations).
func FullName(givenName, familyName string) string {
	given := strings.TrimSpace(givenName)
	family := strings.TrimSpace(familyName)
	switch {
	case given != "" && family != "":
		return given + " " + family
	case given != "":
		return given
	case family != "":
		return family
	default:
		return ""
	}
}

// sha256Hex returns the hex-encoded SHA-256 digest of s. Useful for nonce
// derivation when the caller wants to bind the ID token nonce to a server state.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

var _ = sha256Hex // reserved for future nonce-binding use
