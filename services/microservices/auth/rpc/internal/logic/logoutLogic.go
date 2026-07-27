package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogoutLogic) Logout(in *auth.LogoutRequest) (*auth.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "LogoutLogic.Logout")
	defer span.End()

	l.Infof("Logout attempt")

	if in == nil || in.AccessToken == "" {
		l.Errorf("Logout validation failed: access token is required")
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}

	claims, err := l.svcCtx.TokenMaker.VerifyAccessToken(ctx, in.AccessToken)
	if err != nil {
		l.Errorf("Logout failed to verify token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	// Revoke the access token by value. This is best-effort: the token expires
	// anyway, and a failure here does not compromise the session revocation
	// below. We log but do not fail logout on access-token revocation failure.
	err = l.svcCtx.TokenMaker.RevokeAccessToken(ctx, in.AccessToken)
	if err != nil {
		l.Errorf("Logout failed to revoke access token: %v", err)
	}

	// Revoke the entire session so any refresh token (including copies) is
	// rejected on the next refresh attempt. The TTL covers the refresh token's
	// max lifetime so the Redis entry is cleaned up automatically.
	//
	// This is the CRITICAL security operation: if it fails, the session is NOT
	// revoked and a copied refresh token could still refresh. We must NOT claim
	// successful logout in that case — return an error so the client knows the
	// session is still active and can retry.
	sessionTTL := l.svcCtx.Config.JWT.RefreshExpiryDuration
	if err := l.svcCtx.TokenMaker.RevokeSession(ctx, claims.SessionID, sessionTTL); err != nil {
		l.Errorf("Logout failed to revoke session %s: %v (session is NOT revoked — returning error)", claims.SessionID, err)
		return nil, status.Error(codes.Internal, "failed to revoke session")
	}

	// Defense-in-depth: also revoke the refresh token by value if provided.
	// This is best-effort: the session revocation above is the primary guard.
	if in.RefreshToken != "" {
		if err := l.svcCtx.TokenMaker.RevokeRefreshToken(ctx, in.RefreshToken); err != nil {
			l.Errorf("Logout failed to revoke refresh token by value (session already revoked, continuing): %v", err)
		}
	}

	l.Infof("Logout successful for user %s, session %s revoked", claims.Subject, claims.SessionID)

	return &auth.EmptyResponse{}, nil
}
