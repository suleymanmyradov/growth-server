package auth

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Gateway auth error vocabulary.
//
// The gateway auth logic performs HTTP-facing input validation before calling
// the auth RPC, and re-wraps a few RPC errors into user-friendly messages.
// Every string returned in a gRPC status error from this package is defined
// here as a Msg* constant so the logic and tests share a single source of
// truth and messages cannot silently drift between files.
//
// This vocabulary is intentionally separate from the auth RPC service's
// vocabulary (services/microservices/auth/rpc/internal/logic): the gateway
// speaks to end users on the HTTP API, the auth RPC speaks to internal gRPC
// callers, and the two legitimately use different wording for the same
// condition (e.g. "Valid email is required" here vs "Email is required" in the
// RPC).
//
// Usage at call sites:
//   - Prefer returning a pre-built sentinel (Err*) for reused pairings.
//   - For one-off pairings use a helper with a message constant.

// Message constants — the string vocabulary.
const (
	MsgValidEmailRequired        = "Valid email is required"
	MsgPasswordRequired          = "Password is required"
	MsgInvalidEmailOrPassword    = "Invalid email or password"
	MsgUsernameRequired          = "Username is required"
	MsgUsernameFormat            = "Username must be lowercase, start with a letter, and only contain letters, numbers, underscores, or hyphens"
	MsgPasswordStrength          = "Password must be at least 8 characters with uppercase, lowercase, number and special character"
	MsgFullNameRequired          = "Full name is required"
	MsgTokenRequired             = "Token is required"
	MsgTokenAndStrongPasswordReq = "Valid token and a strong new password are required"
	MsgAuthorizationCodeRequired = "Authorization code is required"
	MsgIdentityTokenRequired     = "Identity token is required"
)

// Sentinel gRPC errors for the most reused (code, message) pairings.
var (
	ErrValidEmailRequired = status.Error(codes.InvalidArgument, MsgValidEmailRequired)
)

// err* helpers pair a gRPC code with a message constant.
func errInvalidArgument(msg string) error { return status.Error(codes.InvalidArgument, msg) }
func errUnauthenticated(msg string) error { return status.Error(codes.Unauthenticated, msg) }
