package mdpropagate

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
)

// Outgoing appends the Principal's identity to outgoing gRPC metadata.
// Only appends non-empty values to avoid sending meaningless metadata.
// Call this at most once per outgoing RPC to avoid duplicate metadata entries.
func Outgoing(ctx context.Context) context.Context {
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return ctx
	}

	pairs := []string{}
	if p.UserID != "" {
		pairs = append(pairs, MDUserID, p.UserID)
	}
	if p.Username != "" {
		pairs = append(pairs, MDUsername, p.Username)
	}
	if len(p.Roles) > 0 {
		// Filter empty strings from roles
		roles := filterEmpty(p.Roles)
		if len(roles) > 0 {
			pairs = append(pairs, MDRoles, strings.Join(roles, ","))
		}
	}
	if p.SessionID != "" {
		pairs = append(pairs, MDSessionID, p.SessionID)
	}

	if len(pairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

// PrincipalFromMetadata extracts Principal from incoming gRPC metadata.
// Returns an error if required metadata is missing.
func PrincipalFromMetadata(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	userIDs := md.Get(MDUserID)
	if len(userIDs) == 0 || userIDs[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	var roles []string
	if rs := md.Get(MDRoles); len(rs) > 0 && rs[0] != "" {
		roles = filterEmpty(strings.Split(rs[0], ","))
	}

	var sessionID string
	if s := md.Get(MDSessionID); len(s) > 0 && s[0] != "" {
		sessionID = s[0]
	}

	var username string
	if u := md.Get(MDUsername); len(u) > 0 && u[0] != "" {
		username = u[0]
	}

	p := principal.Principal{
		UserID:    userIDs[0],
		Username:  username,
		Roles:     roles,
		SessionID: sessionID,
	}
	return principal.WithPrincipal(ctx, p), nil
}

// UnaryServerInterceptor returns a gRPC unary server interceptor that extracts
// the Principal from incoming metadata and stores it in the context.
// Returns Unauthenticated error if required metadata is missing.
func UnaryServerInterceptor() func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, err := PrincipalFromMetadata(ctx)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// UnaryServerInterceptorOptional returns a gRPC unary server interceptor that extracts
// the Principal from incoming metadata if present, but never errors.
// Use this for endpoints that may be called without authentication.
func UnaryServerInterceptorOptional() func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, _ = PrincipalFromMetadata(ctx) // Ignore error, proceed without principal
		return handler(ctx, req)
	}
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that appends
// the Principal's identity to outgoing metadata before each RPC call.
// Use this to automatically propagate authentication context from gateway to downstream services.
func UnaryClientInterceptor() func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = Outgoing(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// filterEmpty removes empty strings from a slice.
func filterEmpty(ss []string) []string {
	var result []string
	for _, s := range ss {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}
