package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/settings"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSettingsLogic {
	return &UpdateSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateSettingsLogic) UpdateSettings(in *settings.UpdateSettingsRequest) (*settings.UpdateSettingsResponse, error) {
	// todo: add your logic here and delete this line

	return &settings.UpdateSettingsResponse{}, nil
}
