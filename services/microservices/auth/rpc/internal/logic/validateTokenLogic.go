package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
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
	if in == nil || in.AccessToken == "" {
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}

	claims, err := l.svcCtx.TokenMaker.VerifyAccessToken(l.ctx, in.AccessToken)
	if err != nil {
		l.Errorf("failed to verify token: %v", err)
		return &auth.ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(l.ctx, claims.Subject)
	if err != nil {
		l.Errorf("failed to get user: %v", err)
		return &auth.ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	return &auth.ValidateTokenResponse{
		Valid:    true,
		UserId:   user.ID.String(),
		Username: user.Username,
	}, nil
}
