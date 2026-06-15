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

type VerifyAccessTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyAccessTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyAccessTokenLogic {
	return &VerifyAccessTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyAccessTokenLogic) VerifyAccessToken(in *auth.VerifyAccessTokenRequest) (*auth.VerifyAccessTokenResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "VerifyAccessTokenLogic.VerifyAccessToken")
	defer span.End()

	l.Infof("VerifyAccessToken attempt")

	if in == nil || in.AccessToken == "" {
		l.Errorf("VerifyAccessToken validation failed: access token is required")
		return nil, status.Error(codes.Unauthenticated, "access token is required")
	}

	claims, err := l.svcCtx.TokenMaker.VerifyAccessToken(ctx, in.AccessToken)
	if err != nil {
		l.Errorf("VerifyAccessToken failed to verify access token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
	}

	// Verify the user still exists and is active
	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, claims.Subject)
	if err != nil {
		l.Errorf("VerifyAccessToken failed to get user %s: %v", claims.Subject, err)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
	}

	var roles []string
	if claims.Roles != nil {
		roles = claims.Roles
	}

	l.Infof("VerifyAccessToken successful for user %s", user.ID)

	return &auth.VerifyAccessTokenResponse{
		UserId:    user.ID.String(),
		Username:  user.Username,
		Roles:     roles,
		SessionId: claims.SessionID.String(),
		ExpiresAt: claims.ExpiresAt.Unix(),
	}, nil
}
