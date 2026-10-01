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

// UnaryServerInterceptorSkipping behaves like UnaryServerInterceptor except
// for the listed full method names (e.g. "/notifications.Notifications/
// UnsubscribeEmail"), which bypass principal extraction entirely. Use for
// endpoints authenticated by other means — e.g. token-authenticated
// unsubscribe links — whose callers carry no user JWT. Service auth (s2s)
// still applies: only internal services can invoke the exempted methods.
func UnaryServerInterceptorSkipping(verifier TokenVerifier, skipMethods ...string) grpc.UnaryServerInterceptor {
	skip := make(map[string]struct{}, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = struct{}{}
	}
	verify := UnaryServerInterceptor(verifier)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if _, ok := skip[info.FullMethod]; ok {
			return handler(ctx, req)
		}
		return verify(ctx, req, info, handler)
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

// StreamClientInterceptor returns a gRPC stream client interceptor that appends
// the raw JWT Authorization header to outgoing metadata before each streaming
// RPC call — the streaming counterpart of UnaryClientInterceptor.
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = Outgoing(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor that
// verifies the propagated JWT and injects the Principal into the stream's
// context, so streaming handlers resolve identity from the verified token
// rather than trusting request fields. Health and reflection methods are
// skipped, matching the s2s interceptor's exemptions.
func StreamServerInterceptor(verifier TokenVerifier) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if skipMethod(info.FullMethod) {
			return handler(srv, ss)
		}
		ctx, err := PrincipalFromMetadata(ss.Context(), verifier)
		if err != nil {
			return err
		}
		return handler(srv, &principalServerStream{ServerStream: ss, ctx: ctx})
	}
}

// principalServerStream replaces the stream context with the principal-bearing
// one produced by StreamServerInterceptor.
type principalServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *principalServerStream) Context() context.Context { return s.ctx }

func skipMethod(method string) bool {
	return strings.HasPrefix(method, "/grpc.health") ||
		strings.HasPrefix(method, "/grpc.reflection") ||
		strings.Contains(method, "Reflection")
}
