package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/settings"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePreferencesLogic {
	return &UpdatePreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdatePreferencesLogic) UpdatePreferences(in *settings.UpdatePreferencesRequest) (*settings.UpdatePreferencesResponse, error) {
	// todo: add your logic here and delete this line

	return &settings.UpdatePreferencesResponse{}, nil
}
