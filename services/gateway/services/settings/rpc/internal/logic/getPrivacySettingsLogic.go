package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/settings/rpc/settings"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPrivacySettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPrivacySettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPrivacySettingsLogic {
	return &GetPrivacySettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Privacy settings
func (l *GetPrivacySettingsLogic) GetPrivacySettings(in *settings.GetPrivacySettingsRequest) (*settings.GetPrivacySettingsResponse, error) {
	// todo: add your logic here and delete this line

	return &settings.GetPrivacySettingsResponse{}, nil
}
