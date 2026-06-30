// Package google provides a minimal helper for the Google OAuth authorization
// code flow: exchange the code for tokens and fetch the verified user info
// (subject, email, name, avatar). It uses Google's userinfo endpoint over TLS
// rather than manually verifying the id_token JWT, which keeps the dependency
// surface small while remaining secure (the access token comes directly from
// Google's token endpoint).
package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

const userinfoEndpoint = "https://www.googleapis.com/oauth2/v3/userinfo"

// UserInfo is the subset of Google profile claims we need for account linking.
type UserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// Config holds the Google OAuth client credentials and redirect URI.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Endpoint is Google's OAuth 2. endpoints (kept here to avoid importing
// golang.org/x/oauth2/google which pulls in AppEngine helpers).
var Endpoint = oauth2.Endpoint{
	AuthURL:       "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL:      "https://oauth2.googleapis.com/token",
	DeviceAuthURL: "https://oauth2.googleapis.com/device/code",
}

// Scopes we request (openid for subject, email for email, profile for name/picture).
var Scopes = []string{"openid", "email", "profile"}

// AuthCodeURL returns the URL the browser should be redirected to in order to
// start the Google sign-in flow. state should be an unguessable value stored
// server-side and verified on callback.
func (c Config) AuthCodeURL(state string) string {
	cfg := c.oauth2Config()
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("prompt", "select_account"))
}

// ExchangeCode exchanges the authorization code returned by Google for user
// info. The redirect URI must match the one used when building AuthCodeURL.
func (c Config) ExchangeCode(ctx context.Context, code, redirectURI string) (UserInfo, error) {
	cfg := c.oauth2Config()
	cfg.RedirectURL = redirectURI

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return UserInfo{}, fmt.Errorf("google: exchange code: %w", err)
	}

	// Prefer the id_token if present (avoids an extra round trip), but fall back
	// to the userinfo endpoint for robustness.
	if idToken, ok := token.Extra("id_token").(string); ok && idToken != "" {
		if ui, err := parseIDTokenClaims(idToken); err == nil {
			return ui, nil
		}
	}

	return fetchUserInfo(ctx, token.AccessToken)
}

func (c Config) oauth2Config() oauth2.Config {
	return oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURI,
		Endpoint:     Endpoint,
		Scopes:       Scopes,
	}
}

func fetchUserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userinfoEndpoint, nil)
	if err != nil {
		return UserInfo{}, fmt.Errorf("google: build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("google: userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return UserInfo{}, fmt.Errorf("google: userinfo status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var ui UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&ui); err != nil {
		return UserInfo{}, fmt.Errorf("google: decode userinfo: %w", err)
	}
	return ui, nil
}

// parseIDTokenClaims extracts claims from the id_token without signature
// verification. This is acceptable ONLY because the token was obtained directly
// from Google's token endpoint over TLS (not from the client). The userinfo
// fallback above is used when the id_token is absent.
func parseIDTokenClaims(idToken string) (UserInfo, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return UserInfo{}, fmt.Errorf("google: malformed id_token")
	}
	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return UserInfo{}, fmt.Errorf("google: decode id_token payload: %w", err)
	}

	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"` // Google encodes this as string or bool
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return UserInfo{}, fmt.Errorf("google: unmarshal id_token claims: %w", err)
	}

	ui := UserInfo{
		Subject:    claims.Sub,
		Email:      claims.Email,
		Name:       claims.Name,
		GivenName:  claims.GivenName,
		FamilyName: claims.FamilyName,
		Picture:    claims.Picture,
	}
	switch v := claims.EmailVerified.(type) {
	case bool:
		ui.EmailVerified = v
	case string:
		ui.EmailVerified = strings.EqualFold(v, "true")
	}
	return ui, nil
}

func base64URLDecode(s string) ([]byte, error) {
	// Pad to a multiple of 4 so URLEncoding (which expects padding) can decode it.
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	return base64.URLEncoding.DecodeString(s)
}
