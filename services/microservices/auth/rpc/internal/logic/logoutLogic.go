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

	_, err := l.svcCtx.TokenMaker.VerifyAccessToken(ctx, in.AccessToken)
	if err != nil {
		l.Errorf("Logout failed to verify token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	err = l.svcCtx.TokenMaker.RevokeAccessToken(ctx, in.AccessToken)
	if err != nil {
		l.Errorf("Logout failed to revoke token: %v", err)
	}

	l.Infof("Logout successful")

	return &auth.EmptyResponse{}, nil
}
