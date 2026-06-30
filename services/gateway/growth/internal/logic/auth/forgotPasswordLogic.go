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

type ForgotPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewForgotPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgotPasswordLogic {
	return &ForgotPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ForgotPasswordLogic) ForgotPassword(req *types.ForgotPasswordRequest) (*types.EmptyResponse, error) {
	if req == nil || !validator.IsValidEmail(req.Email) {
		return nil, status.Error(codes.InvalidArgument, "valid email is required")
	}

	_, err := l.svcCtx.AuthRpc.ForgotPassword(l.ctx, &authservice.ForgotPasswordRequest{
		Email: req.Email,
	})
	if err != nil {
		return nil, err
	}

	// Always return success to avoid leaking whether the email exists.
	return &types.EmptyResponse{}, nil
}
