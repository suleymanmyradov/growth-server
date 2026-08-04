package logic

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Auth RPC error vocabulary.
//
// Every user-facing string returned in a gRPC status error from this package
// is defined here as a Msg* constant, so the logic layer and its tests share a
// single source of truth and messages cannot silently drift between files.
//
// Usage at call sites:
//   - Prefer returning a pre-built sentinel (Err*) for reused (code, message)
//     pairings — e.g. `return nil, ErrInvalidCredentials`.
//   - For one-off pairings, or where the same message appears under different
//     codes, use a helper with a message constant — e.g.
//     `return nil, errInternal(MsgFailedDeleteUser)`.
//
// Note: these sentinels are convenience values for returning, not for
// identity comparison via errors.Is across independently-constructed status
// errors. Tests assert via assertGrpcError (code + message), which works
// regardless of whether the error was a sentinel or a helper call. A returned
// sentinel IS comparable by == / errors.Is to the same package var, since the
// caller receives the exact same error value.

// Message constants — the string vocabulary.
const (
	// Request validation
	MsgRequestIsNil              = "Request is nil"
	MsgEmailIsRequired           = "Email is required"
	MsgInvalidEmailFormat        = "Invalid email format"
	MsgEmailAndPasswordRequired  = "Email and password are required"
	MsgUsernameEmailPasswordReq  = "Username, email and password are required"
	MsgUsernameFormat            = "Username must be lowercase, start with a letter, and only contain letters, numbers, underscores, or hyphens"
	MsgPasswordStrength          = "Password must be at least 8 characters and contain uppercase, lowercase, number, and special character"
	MsgTokenIsRequired           = "Token is required"
	MsgTokenAndPasswordRequired  = "Token and new password are required"
	MsgAccessTokenRequired       = "Access token is required"
	MsgRefreshTokenRequired      = "Refresh token is required"
	MsgUserIdRequired            = "User ID is required"
	MsgUserIdOldNewPasswordReq   = "User ID, old password and new password are required"
	MsgInvalidUserId             = "Invalid user ID"
	MsgMissingPrincipal          = "Missing principal"
	MsgAuthorizationCodeRequired = "Authorization code is required"
	MsgIdentityTokenRequired     = "Identity token is required"
	MsgRedirectURINotAllowed     = "Redirect URI not allowed"

	// Credentials / auth state
	MsgInvalidCredentials           = "Invalid credentials"
	MsgEmailNotVerified             = "Email not verified"
	MsgInvalidOrExpiredAccessToken  = "Invalid or expired access token"
	MsgInvalidOrExpiredVerifToken   = "Invalid or expired verification token"
	MsgInvalidOrExpiredResetToken   = "Invalid or expired reset token"
	MsgInvalidOrExpiredRefreshToken = "Invalid or expired refresh token"
	MsgSessionRevoked               = "Session revoked"
	MsgInvalidToken                 = "Invalid token"
	MsgInvalidOldPassword           = "Invalid old password"
	MsgFailedAuthGoogle             = "Failed to authenticate with Google"
	MsgFailedAuthApple              = "Failed to authenticate with Apple"
	MsgIncompleteGoogleProfile      = "Incomplete Google profile"
	MsgIncompleteAppleProfile       = "Incomplete Apple profile"

	// Precondition / config
	MsgGoogleNotConfigured     = "Google sign-in is not configured"
	MsgAppleNotConfigured      = "Apple sign-in is not configured"
	MsgNoPasswordSetForAccount = "No password set for this account"
	MsgPleaseWaitBeforeResend  = "Please wait before requesting another verification email"

	// Resource state
	MsgUserNotFound      = "User not found"
	MsgUserAlreadyExists = "User already exists"

	// Internal failures
	MsgFailedProcessPassword       = "Failed to process password"
	MsgFailedCreateUser            = "Failed to create user"
	MsgFailedGenerateVerifToken    = "Failed to generate verification token"
	MsgFailedSendVerificationEmail = "Failed to send verification email"
	MsgFailedProcessPasswordReset  = "Failed to process password reset"
	MsgFailedGenerateAccessToken   = "Failed to generate access token"
	MsgFailedGenerateRefreshToken  = "Failed to generate refresh token"
	MsgFailedLoadLinkedUser        = "Failed to load linked user"
	MsgFailedLinkGoogleAccount     = "Failed to link Google account"
	MsgFailedLinkAppleAccount      = "Failed to link Apple account"
	MsgFailedValidateToken         = "Failed to validate token"
	MsgInvalidVerificationToken    = "Invalid verification token"
	MsgFailedVerifyEmail           = "Failed to verify email"
	MsgFailedRevokeSession         = "Failed to revoke session"
	MsgFailedValidateResetToken    = "Failed to validate reset token"
	MsgFailedProcessNewPassword    = "Failed to process new password"
	MsgFailedUpdatePassword        = "Failed to update password"
	MsgFailedDeleteUser            = "Failed to delete user"
	MsgFailedValidateSession       = "Failed to validate session"
	MsgFailedUpdateUser            = "Failed to update user"
	MsgFailedUpdateProfile         = "Failed to update profile"
)

// Sentinel gRPC errors for the most reused (code, message) pairings.
// Return these directly at call sites.
var (
	ErrInvalidCredentials   = status.Error(codes.Unauthenticated, MsgInvalidCredentials)
	ErrEmailNotVerified     = status.Error(codes.PermissionDenied, MsgEmailNotVerified)
	ErrUserNotFound         = status.Error(codes.NotFound, MsgUserNotFound)
	ErrUserAlreadyExists    = status.Error(codes.AlreadyExists, MsgUserAlreadyExists)
	ErrFailedGenAccessToken = status.Error(codes.Internal, MsgFailedGenerateAccessToken)
	ErrFailedGenRefreshTok  = status.Error(codes.Internal, MsgFailedGenerateRefreshToken)
)

// err* helpers pair a gRPC code with a message constant. Use these for one-off
// (code, message) pairings or when the same message appears under different
// codes (e.g. MsgAccessTokenRequired is InvalidArgument in ValidateToken but
// Unauthenticated in VerifyAccessToken). Codes covered by sentinels above
// (PermissionDenied, NotFound, AlreadyExists) do not need helpers here.
func errInvalidArgument(msg string) error    { return status.Error(codes.InvalidArgument, msg) }
func errUnauthenticated(msg string) error    { return status.Error(codes.Unauthenticated, msg) }
func errFailedPrecondition(msg string) error { return status.Error(codes.FailedPrecondition, msg) }
func errResourceExhausted(msg string) error  { return status.Error(codes.ResourceExhausted, msg) }
func errInternal(msg string) error           { return status.Error(codes.Internal, msg) }
