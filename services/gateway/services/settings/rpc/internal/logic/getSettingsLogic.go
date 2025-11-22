package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/settings"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSettingsLogic {
	return &GetSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// User settings
func (l *GetSettingsLogic) GetSettings(in *settings.GetSettingsRequest) (*settings.GetSettingsResponse, error) {
	// todo: add your logic here and delete this line

	return &settings.GetSettingsResponse{}, nil
}
