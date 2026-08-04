package auth

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Adminway auth error vocabulary.
//
// The adminway auth logic validates admin credentials and issues JWTs for the
// admin panel. Every string returned in a gRPC status error from this package
// is defined here as a Msg* constant so the logic and tests share a single
// source of truth and messages cannot silently drift.
//
// The handlers call go-zero's httpx.ErrorCtx, which is wired to a custom
// SetErrorHandlerCtx installed in adminway's main(). That handler maps gRPC
// status codes → HTTP status codes and sanitizes the message, so returning
// status.Error(codes.X, MsgY) here produces a proper HTTP response (e.g. 401
// for Unauthenticated, 400 for InvalidArgument) instead of a raw 500 with
// internal error text.
//
// Usage at call sites:
//   - Prefer returning a pre-built sentinel (Err*) for reused pairings.
//   - For one-off pairings use a helper with a message constant.

// Message constants — the string vocabulary.
const (
	MsgEmailAndPasswordRequired = "Email and password are required"
	MsgInvalidCredentials       = "Invalid email or password"
	MsgEmailAlreadyExists       = "An admin with this email already exists"
	MsgFailedHashPassword       = "Failed to process password"
	MsgFailedCreateAdmin        = "Failed to create admin user"
	MsgFailedGenAccessToken     = "Failed to generate access token"
	MsgFailedGenRefreshToken    = "Failed to generate refresh token"
	MsgInvalidOrExpiredRefresh  = "Invalid or expired refresh token"
	MsgAdminNotFound            = "Admin user not found"
)

// Sentinel gRPC errors for the most reused (code, message) pairings.
var (
	ErrInvalidCredentials    = status.Error(codes.Unauthenticated, MsgInvalidCredentials)
	ErrEmailAlreadyExists    = status.Error(codes.AlreadyExists, MsgEmailAlreadyExists)
	ErrInvalidExpiredRefresh = status.Error(codes.Unauthenticated, MsgInvalidOrExpiredRefresh)
	ErrAdminNotFound         = status.Error(codes.NotFound, MsgAdminNotFound)
	ErrFailedGenAccessToken  = status.Error(codes.Internal, MsgFailedGenAccessToken)
	ErrFailedGenRefreshToken = status.Error(codes.Internal, MsgFailedGenRefreshToken)
)

// err* helpers pair a gRPC code with a message constant.
func errInvalidArgument(msg string) error { return status.Error(codes.InvalidArgument, msg) }
func errInternal(msg string) error        { return status.Error(codes.Internal, msg) }
