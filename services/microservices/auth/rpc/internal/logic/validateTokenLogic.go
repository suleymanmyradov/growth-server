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

type ValidateTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateTokenLogic {
	return &ValidateTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateTokenLogic) ValidateToken(in *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ValidateTokenLogic.ValidateToken")
	defer span.End()

	l.Infof("ValidateToken attempt")

	if in == nil || in.AccessToken == "" {
		l.Errorf("ValidateToken validation failed: access token is required")
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}

	claims, err := l.svcCtx.TokenMaker.VerifyAccessToken(ctx, in.AccessToken)
	if err != nil {
		l.Errorf("ValidateToken failed to verify token: %v", err)
		return &auth.ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, claims.Subject)
	if err != nil {
		l.Errorf("ValidateToken failed to get user %s: %v", claims.Subject, err)
		return &auth.ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	l.Infof("ValidateToken successful for user %s", claims.Subject)

	return &auth.ValidateTokenResponse{
		Valid:    true,
		UserId:   user.ID.String(),
		Username: user.Username,
	}, nil
}
