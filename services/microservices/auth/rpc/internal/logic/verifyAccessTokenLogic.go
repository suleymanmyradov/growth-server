package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
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
	if in == nil || in.AccessToken == "" {
		return nil, status.Error(codes.Unauthenticated, "access token is required")
	}

	claims, err := l.svcCtx.TokenMaker.VerifyAccessToken(l.ctx, in.AccessToken)
	if err != nil {
		l.Errorf("failed to verify access token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
	}

	var roles []string
	if claims.Roles != nil {
		roles = claims.Roles
	}

	return &auth.VerifyAccessTokenResponse{
		UserId:    claims.Subject.String(),
		Username:  claims.Username,
		Roles:     roles,
		SessionId: claims.SessionID.String(),
		ExpiresAt: claims.ExpiresAt.Unix(),
	}, nil
}
