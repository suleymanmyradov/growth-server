package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordLogic {
	return &ResetPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetPasswordLogic) ResetPassword(in *auth.ResetPasswordRequest) (*auth.EmptyResponse, error) {
	// todo: add your logic here and delete this line

	return &auth.EmptyResponse{}, nil
}
