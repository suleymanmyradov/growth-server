// ID token verification for the native Google sign-in flow.
//
// Native clients (iOS/Android OAuth client IDs) are public: expo-auth-session
// performs the authorization request with PKCE and exchanges the code itself —
// no client secret is involved. The resulting id_token is sent to the backend
// and verified here by signature against Google's JWKS, mirroring the Apple
// flow in pkg/oauth/apple. The web flow is unchanged: it still sends an
// authorization code that Config.ExchangeCode redeems with the web client's
// secret.
package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// googleJWKSURL is Google's published public key set for ID token
	// verification.
	googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
	// jwksCacheTTL bounds how long fetched keys are cached in memory.
	jwksCacheTTL = 15 * time.Minute
)

// googleIssuers are the accepted `iss` values. Google issues both variants.
var googleIssuers = map[string]bool{
	"accounts.google.com":         true,
	"https://accounts.google.com": true,
}

// IDTokenVerifier verifies Google ID tokens by signature against Google's
// JWKS. Keys are cached in memory for jwksCacheTTL.
type IDTokenVerifier struct {
	// allowedAudiences is the set of accepted `aud` values — the web, iOS, and
	// Android OAuth client IDs of this Google Cloud project.
	allowedAudiences map[string]bool
	httpClient       *http.Client
	jwksURL          string

	cachedKeys map[string]*rsa.PublicKey
	jwksExpiry time.Time
}

// NewIDTokenVerifier returns a verifier accepting tokens whose `aud` is any of
// allowedAudiences. The HTTP client may be nil (http.DefaultClient is used);
// injecting a client lets tests serve a mock JWKS endpoint.
func NewIDTokenVerifier(allowedAudiences []string, httpClient *http.Client) *IDTokenVerifier {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	allowed := make(map[string]bool, len(allowedAudiences))
	for _, a := range allowedAudiences {
		if a != "" {
			allowed[a] = true
		}
	}
	return &IDTokenVerifier{
		allowedAudiences: allowed,
		httpClient:       httpClient,
		jwksURL:          googleJWKSURL,
	}
}

// googleIDTokenClaims is the claim set carried by a Google ID token. Standard
// claims are validated by the JWT library where possible; issuer and audience
// are checked manually because both accept a set of values.
type googleIDTokenClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // Google encodes as string or bool
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// VerifyIDToken verifies the signature, issuer, audience, and expiry of a
// Google ID token and returns the profile claims needed for account linking.
func (v *IDTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (UserInfo, error) {
	if idToken == "" {
		return UserInfo{}, fmt.Errorf("google: id token is required")
	}
	if len(v.allowedAudiences) == 0 {
		return UserInfo{}, fmt.Errorf("google: no allowed audiences configured")
	}

	keys, err := v.fetchJWKS(ctx)
	if err != nil {
		return UserInfo{}, fmt.Errorf("google: fetch jwks: %w", err)
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
	)

	var claims googleIDTokenClaims
	parsed, err := parser.ParseWithClaims(idToken, &claims, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("google: id token missing kid header")
		}
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("google: no matching key for kid %s", kid)
		}
		return key, nil
	})
	if err != nil {
		return UserInfo{}, fmt.Errorf("google: verify id token: %w", err)
	}
	if !parsed.Valid {
		return UserInfo{}, fmt.Errorf("google: id token is not valid")
	}

	if !googleIssuers[claims.Issuer] {
		return UserInfo{}, fmt.Errorf("google: unexpected issuer %q", claims.Issuer)
	}
	if !audienceAllowed(claims.Audience, v.allowedAudiences) {
		return UserInfo{}, fmt.Errorf("google: audience not allowed")
	}
	if claims.Subject == "" {
		return UserInfo{}, fmt.Errorf("google: id token missing sub claim")
	}
	if claims.Email == "" {
		return UserInfo{}, fmt.Errorf("google: id token missing email claim")
	}

	ui := UserInfo{
		Subject:    claims.Subject,
		Email:      claims.Email,
		Name:       claims.Name,
		GivenName:  claims.GivenName,
		FamilyName: claims.FamilyName,
		Picture:    claims.Picture,
	}
	switch ev := claims.EmailVerified.(type) {
	case bool:
		ui.EmailVerified = ev
	case string:
		ui.EmailVerified = strings.EqualFold(ev, "true")
	}
	return ui, nil
}

// audienceAllowed reports whether any token `aud` value is in the allowlist.
// Google ID tokens carry a single audience (the issuing client ID).
func audienceAllowed(tokenAud jwt.ClaimStrings, allowed map[string]bool) bool {
	for _, a := range tokenAud {
		if allowed[a] {
			return true
		}
	}
	return false
}

// fetchJWKS fetches and parses Google's JWKS, caching the parsed RSA public
// keys keyed by kid until jwksExpiry.
func (v *IDTokenVerifier) fetchJWKS(ctx context.Context) (map[string]*rsa.PublicKey, error) {
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google: jwks status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
		return nil, fmt.Errorf("google: decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		pub, err := jwkToRSAPublicKey(k.N, k.E)
		if err != nil {
			return nil, fmt.Errorf("google: parse jwk kid=%s: %w", k.Kid, err)
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("google: jwks contained no usable RSA keys")
	}

	v.cachedKeys = keys
	v.jwksExpiry = time.Now().Add(jwksCacheTTL)
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
