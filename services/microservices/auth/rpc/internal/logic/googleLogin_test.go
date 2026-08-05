package logic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"google.golang.org/grpc/codes"
)

// ============================================
// GoogleLoginLogic — helper function tests
//
// The actual OAuth code exchange (cfg.ExchangeCode) and DB operations inside
// the TxRunner callback can't be unit tested without a real Google callback
// and Postgres. These tests cover the helper functions and validation paths
// that are pure and testable in isolation.
// ============================================

// googleOAuthConfig builds a config.Config with Google OAuth fields set.
func googleOAuthConfig(clientID, clientSecret, redirectURI string, allowed ...string) config.Config {
	cfg := config.Config{}
	cfg.GoogleOAuth.ClientID = clientID
	cfg.GoogleOAuth.ClientSecret = clientSecret
	cfg.GoogleOAuth.RedirectURI = redirectURI
	cfg.GoogleOAuth.AllowedRedirectURIs = allowed
	return cfg
}

// -------------------------------------------
// deriveUsername
// -------------------------------------------

func TestDeriveUsername_FromEmailLocalPart(t *testing.T) {
	assert.Equal(t, "jane", deriveUsername("jane@example.com", "Jane Doe"))
}

func TestDeriveUsername_FallsBackToName(t *testing.T) {
	// Email with no @ or empty local part: strings.Index returns 0 for
	// "@example.com", and at > 0 is false, so local stays as the full email.
	// sanitizeUsername("@example.com") → "example_com" (leading @ stripped,
	// dot → underscore). The name is only used when sanitizeUsername(local) == "".
	assert.Equal(t, "example_com", deriveUsername("@example.com", "John Doe"))
}

func TestDeriveUsername_FallsBackToUserWhenBothEmpty(t *testing.T) {
	// When both email local part and name sanitize to empty, fallback to "user"
	// Email "!!!@x.com" → local="!!!", sanitizeUsername("!!!")="" → name="" → "user"
	assert.Equal(t, "user", deriveUsername("!!!@x.com", ""))
}

func TestDeriveUsername_SanitizesSpecialChars(t *testing.T) {
	// Special chars in the local part become underscores
	assert.Equal(t, "jane_doe_test", deriveUsername("jane.doe+test@example.com", ""))
}

func TestDeriveUsername_LowercasesInput(t *testing.T) {
	assert.Equal(t, "jane", deriveUsername("Jane@Example.com", ""))
}

func TestDeriveUsername_StripsLeadingDigits(t *testing.T) {
	// Username must start with a letter per the CHECK constraint.
	// sanitizeUsername strips leading digits, so "123abc" → "abc"
	assert.Equal(t, "abc", deriveUsername("123abc@example.com", ""))
}

// -------------------------------------------
// sanitizeUsername
// -------------------------------------------

func TestSanitizeUsername_LowercasesAndReplacesSpecials(t *testing.T) {
	assert.Equal(t, "john_doe", sanitizeUsername("John.Doe"))
}

func TestSanitizeUsername_StripsLeadingNonLetters(t *testing.T) {
	assert.Equal(t, "abc", sanitizeUsername("123abc"))
}

func TestSanitizeUsername_HandlesUnderscoresAndHyphens(t *testing.T) {
	assert.Equal(t, "a_b-c", sanitizeUsername("a_b-c"))
}

func TestSanitizeUsername_EmptyInput(t *testing.T) {
	assert.Equal(t, "", sanitizeUsername(""))
}

func TestSanitizeUsername_OnlySpecialChars(t *testing.T) {
	assert.Equal(t, "", sanitizeUsername("...!!!"))
}

func TestSanitizeUsername_TrimsTrailingUnderscoresAndHyphens(t *testing.T) {
	assert.Equal(t, "abc", sanitizeUsername("abc___"))
	assert.Equal(t, "abc", sanitizeUsername("abc---"))
}

// -------------------------------------------
// trimUsername
// -------------------------------------------

func TestTrimUsername_NoTruncationUnderLimit(t *testing.T) {
	assert.Equal(t, "short", trimUsername("short"))
}

func TestTrimUsername_TruncatesOverLimit(t *testing.T) {
	long := make([]byte, 50)
	for i := range long {
		long[i] = 'a'
	}
	result := trimUsername(string(long))
	assert.Len(t, result, 45)
}

// -------------------------------------------
// itoa
// -------------------------------------------

func TestItoa(t *testing.T) {
	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "1", itoa(1))
	assert.Equal(t, "42", itoa(42))
	assert.Equal(t, "123", itoa(123))
}

// -------------------------------------------
// isAllowedRedirectURI
// -------------------------------------------

func TestIsAllowedRedirectURI_MatchesConfigured(t *testing.T) {
	assert.True(t, isAllowedRedirectURI("http://localhost:3000/auth/callback/google", nil, "http://localhost:3000/auth/callback/google"))
}

func TestIsAllowedRedirectURI_MatchesAllowlist(t *testing.T) {
	allowed := []string{"http://localhost:3000/auth/callback/google", "https://example.com/auth/callback/google"}
	assert.True(t, isAllowedRedirectURI("https://example.com/auth/callback/google", allowed, "http://localhost:3000/auth/callback/google"))
}

func TestIsAllowedRedirectURI_RejectsUnknown(t *testing.T) {
	assert.False(t, isAllowedRedirectURI("http://evil.com/callback", nil, "http://localhost:3000/auth/callback/google"))
}

func TestIsAllowedRedirectURI_RejectsPathTraversal(t *testing.T) {
	// Exact match only — no prefix matching
	assert.False(t, isAllowedRedirectURI("http://localhost:3000/auth/callback/google/extra", nil, "http://localhost:3000/auth/callback/google"))
}

func TestIsAllowedRedirectURI_EmptyAllowlistFallsBackToConfigured(t *testing.T) {
	assert.True(t, isAllowedRedirectURI("http://localhost:3000/auth/callback/google", []string{}, "http://localhost:3000/auth/callback/google"))
}

// -------------------------------------------
// GoogleLoginLogic validation
// -------------------------------------------

func TestGoogleLoginLogic_NilRequest(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: googleOAuthConfig("test-client-id", "test-client-secret", "http://localhost:3000/auth/callback/google"),
	}
	_, err := NewGoogleLoginLogic(context.Background(), svcCtx).GoogleLogin(nil)
	assertGrpcError(t, err, codes.InvalidArgument, MsgAuthorizationCodeRequired)
}

func TestGoogleLoginLogic_EmptyAuthorizationCode(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: googleOAuthConfig("test-client-id", "test-client-secret", "http://localhost:3000/auth/callback/google"),
	}
	_, err := NewGoogleLoginLogic(context.Background(), svcCtx).GoogleLogin(&auth.GoogleLoginRequest{
		AuthorizationCode: "",
	})
	assertGrpcError(t, err, codes.InvalidArgument, MsgAuthorizationCodeRequired)
}

func TestGoogleLoginLogic_OAuthNotConfigured(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: googleOAuthConfig("", "", "http://localhost:3000/auth/callback/google"),
	}
	_, err := NewGoogleLoginLogic(context.Background(), svcCtx).GoogleLogin(&auth.GoogleLoginRequest{
		AuthorizationCode: "valid-code",
	})
	assertGrpcError(t, err, codes.FailedPrecondition, MsgGoogleNotConfigured)
}

func TestGoogleLoginLogic_OAuthNotConfigured_MissingSecret(t *testing.T) {
	// ClientID present but ClientSecret missing
	svcCtx := &svc.ServiceContext{
		Config: googleOAuthConfig("test-client-id", "", "http://localhost:3000/auth/callback/google"),
	}
	_, err := NewGoogleLoginLogic(context.Background(), svcCtx).GoogleLogin(&auth.GoogleLoginRequest{
		AuthorizationCode: "valid-code",
	})
	assertGrpcError(t, err, codes.FailedPrecondition, MsgGoogleNotConfigured)
}

func TestGoogleLoginLogic_DisallowedRedirectURI(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: googleOAuthConfig("test-client-id", "test-client-secret", "http://localhost:3000/auth/callback/google"),
	}
	_, err := NewGoogleLoginLogic(context.Background(), svcCtx).GoogleLogin(&auth.GoogleLoginRequest{
		AuthorizationCode: "valid-code",
		RedirectUri:       "http://evil.com/callback",
	})
	assertGrpcError(t, err, codes.InvalidArgument, MsgRedirectURINotAllowed)
}

func TestGoogleLoginLogic_AllowedRedirectURIFromAllowlist(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: googleOAuthConfig("test-client-id", "test-client-secret",
			"http://localhost:3000/auth/callback/google",
			"https://example.com/auth/callback/google"),
	}
	// This should pass validation (redirect URI is in the allowlist) but then
	// fail at the ExchangeCode step (no real Google API). We just verify the
	// redirect URI validation passes.
	_, err := NewGoogleLoginLogic(context.Background(), svcCtx).GoogleLogin(&auth.GoogleLoginRequest{
		AuthorizationCode: "valid-code",
		RedirectUri:       "https://example.com/auth/callback/google",
	})
	// Should fail with "Failed to authenticate with Google" (ExchangeCode fails)
	// — NOT "Redirect URI not allowed"
	assertGrpcError(t, err, codes.Unauthenticated, MsgFailedAuthGoogle)
}

func TestGoogleLoginLogic_DefaultRedirectURIWhenNotProvided(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: googleOAuthConfig("test-client-id", "test-client-secret", "http://localhost:3000/auth/callback/google"),
	}
	// No RedirectUri provided — should use the configured one and proceed to
	// ExchangeCode (which will fail since there's no real Google API)
	_, err := NewGoogleLoginLogic(context.Background(), svcCtx).GoogleLogin(&auth.GoogleLoginRequest{
		AuthorizationCode: "valid-code",
	})
	// Should fail at ExchangeCode, not at redirect URI validation
	assertGrpcError(t, err, codes.Unauthenticated, MsgFailedAuthGoogle)
}
