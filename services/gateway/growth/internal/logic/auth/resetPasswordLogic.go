// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordLogic {
	return &ResetPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResetPasswordLogic) ResetPassword(req *types.ResetPasswordRequest) (*types.EmptyResponse, error) {
	if req == nil || req.Token == "" || !validator.IsStrongPassword(req.NewPassword) {
		return nil, status.Error(codes.InvalidArgument, "valid token and a strong new password are required")
	}

	_, err := l.svcCtx.AuthRpc.ResetPassword(l.ctx, &authservice.ResetPasswordRequest{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
