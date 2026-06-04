// Package auth provides a unified wiring example showing how to compose
// jwt, mdpropagate, s2s, principal, and authz for secure microservices.
//
// This file is documentation-as-code: copy the patterns into your service main.go
// and gRPC server setup.
//
// Recommended interceptor order (outermost → innermost):
//  1. Recovery / panic handler
//  2. Logging / tracing
//  3. s2s.UnaryServerInterceptor  (verify caller is an internal service)
//  4. mdpropagate.UnaryServerInterceptor  (verify JWT, extract principal)
//  5. authz.Checker.CheckPrincipal  (verify user is still active)
//  6. Rate limiting / quotas
//  7. Business logic handler
//
// Gateway → Service call order:
//  1. Parse HTTP Authorization header at gateway
//  2. Store raw JWT in context via principal.WithToken
//  3. Call downstream RPC with mdpropagate.UnaryClientInterceptor
//     (sends raw JWT in growth-authorization metadata)
//  4. Downstream service verifies s2s signature
//  5. Downstream service verifies JWT via mdpropagate + public key
//  6. Downstream service checks authz
package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/authz"
	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

// ExampleConfig shows how to structure auth configuration safely.
// Use `secret:"true"` tags so configsafe.MaskSecrets will redact sensitive fields.
type ExampleConfig struct {
	JWT jwt.Config `json:"jwt"`
	S2S s2s.Config `json:"s2s"`
}

// ExampleGatewaySetup demonstrates how a go-zero gateway should parse the
// HTTP Authorization header, verify the JWT, and store it in context for
// downstream propagation.
func ExampleGatewaySetup(ctx context.Context, tokenMaker *jwt.TokenMaker, authHeader string) (context.Context, error) {
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) <= len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		return nil, fmt.Errorf("invalid authorization format")
	}

	tokenString := authHeader[len(bearerPrefix):]
	claims, err := tokenMaker.VerifyAccessToken(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	p := principal.Principal{
		UserID:    claims.Subject.String(),
		Username:  claims.Username,
		Roles:     claims.Roles,
		SessionID: claims.SessionID.String(),
	}

	ctx = principal.WithPrincipal(ctx, p)
	ctx = principal.WithToken(ctx, tokenString)
	return ctx, nil
}

// ExampleServiceServerOptions returns the recommended gRPC server interceptors
// for a downstream microservice.
func ExampleServiceServerOptions(cfg ExampleConfig, tokenVerifier mdpropagate.TokenVerifier, authzChecker *authz.Checker) []grpc.ServerOption {
	// Validate secrets at startup — fail closed
	if err := cfg.S2S.MustValidate(); err != nil {
		logx.Must(err)
	}
	if cfg.JWT.Secret == "" {
		logx.Must(fmt.Errorf("jwt secret is required"))
	}

	s2sInterceptor := s2s.UnaryServerInterceptor(s2s.Config{Secret: cfg.S2S.Secret})
	authInterceptor := mdpropagate.UnaryServerInterceptor(tokenVerifier)

	// Optional: chain authz check after principal extraction
	authzInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := authzChecker.CheckPrincipal(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}

	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			s2sInterceptor,
			authInterceptor,
			authzInterceptor,
		),
	}
}

// ExampleServiceClientOptions returns the recommended zrpc client options
// for calls from gateway to downstream services.
// Using zrpc.WithUnaryClientInterceptor preserves go-zero built-in interceptors
// (trace, prometheus, breaker, timeout, duration).
func ExampleServiceClientOptions(cfg ExampleConfig) []zrpc.ClientOption {
	if err := cfg.S2S.MustValidate(); err != nil {
		logx.Must(err)
	}

	s2sClient := s2s.UnaryClientInterceptor(s2s.Config{Secret: cfg.S2S.Secret})
	authClient := mdpropagate.UnaryClientInterceptor()

	return []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(s2sClient),
		zrpc.WithUnaryClientInterceptor(authClient),
	}
}

// ExampleSafeLogging demonstrates using configsafe to log configuration
// without leaking secrets.
func ExampleSafeLogging(cfg ExampleConfig) {
	// Never log cfg directly; always mask first.
	safe := configsafe.MaskSecrets(cfg)
	logx.Infow("service starting", logx.Field("config", safe))
}

// ExampleAuthzLookup is a template for the Lookup function passed to authz.NewChecker.
func ExampleAuthzLookup(ctx context.Context, userID uuid.UUID) (authz.UserStatus, error) {
	// TODO: replace with actual DB query
	// Example SQL: SELECT status FROM users WHERE id = $1
	return authz.StatusActive, nil
}
