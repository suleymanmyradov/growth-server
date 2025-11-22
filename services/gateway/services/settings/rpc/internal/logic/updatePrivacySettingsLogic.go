package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/settings"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePrivacySettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePrivacySettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePrivacySettingsLogic {
	return &UpdatePrivacySettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdatePrivacySettingsLogic) UpdatePrivacySettings(in *settings.UpdatePrivacySettingsRequest) (*settings.UpdatePrivacySettingsResponse, error) {
	// todo: add your logic here and delete this line

	return &settings.UpdatePrivacySettingsResponse{}, nil
}
