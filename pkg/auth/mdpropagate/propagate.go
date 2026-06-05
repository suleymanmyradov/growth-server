package mdpropagate

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
)

// TokenVerifier is the interface required by PrincipalFromMetadata to verify
// the propagated JWT. Implementations must return ErrInvalidToken for any
// verification failure.
type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, tokenString string) (*jwt.TokenClaims, error)
}

// Outgoing appends the raw JWT Authorization header to outgoing gRPC metadata.
// This propagates the original bearer token so downstream services can verify
// it independently with their own public key. It never sends plain-text identity
// claims that could be forged by a malicious client.
func Outgoing(ctx context.Context) context.Context {
	token, ok := principal.TokenFrom(ctx)
	if !ok || token == "" {
		return ctx
	}

	return metadata.AppendToOutgoingContext(ctx, MDAuthorization, "Bearer "+token)
}

// PrincipalFromMetadata extracts the Principal from incoming gRPC metadata by
// reading the propagated Authorization header and verifying the JWT with the
// provided TokenVerifier. This ensures downstream services do not blindly
// trust gateway headers.
func PrincipalFromMetadata(ctx context.Context, verifier TokenVerifier) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get(MDAuthorization)
	if len(auth) == 0 || auth[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "missing authorization")
	}

	token := strings.TrimPrefix(auth[0], "Bearer ")
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}

	claims, err := verifier.VerifyAccessToken(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	p := principal.Principal{
		UserID:    claims.Subject.String(),
		Username:  claims.Username,
		Roles:     claims.Roles,
		SessionID: claims.SessionID.String(),
	}
	return principal.WithPrincipal(ctx, p), nil
}

// UnaryServerInterceptor returns a gRPC unary server interceptor that extracts
// the Principal from incoming metadata by verifying the propagated JWT.
// Returns Unauthenticated error if required metadata is missing or verification fails.
func UnaryServerInterceptor(verifier TokenVerifier) func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, err := PrincipalFromMetadata(ctx, verifier)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// UnaryServerInterceptorOptional returns a gRPC unary server interceptor that extracts
// the Principal from incoming metadata if present, but never errors.
// Use this only for endpoints that may be called without authentication.
func UnaryServerInterceptorOptional(verifier TokenVerifier) func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// PrincipalFromMetadata returns nil on failure; preserve the original
		// context so downstream code (e.g. Redis calls with context timeouts)
		// does not panic on a nil parent.
		if principalCtx, err := PrincipalFromMetadata(ctx, verifier); err == nil && principalCtx != nil {
			ctx = principalCtx
		}
		return handler(ctx, req)
	}
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that appends
// the raw JWT Authorization header to outgoing metadata before each RPC call.
// Use this to automatically propagate authentication context from gateway to downstream services.
func UnaryClientInterceptor() func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = Outgoing(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
