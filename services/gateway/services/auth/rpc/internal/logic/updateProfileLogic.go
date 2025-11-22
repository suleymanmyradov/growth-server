package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProfileLogic {
	return &UpdateProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Profile management
func (l *UpdateProfileLogic) UpdateProfile(in *auth.UpdateProfileRequest) (*auth.UpdateProfileResponse, error) {
	// todo: add your logic here and delete this line

	return &auth.UpdateProfileResponse{}, nil
}
