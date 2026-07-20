package sitesettingslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ListAllSiteSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAllSiteSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAllSiteSettingsLogic {
	return &ListAllSiteSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListAllSiteSettingsLogic) ListAllSiteSettings(in *client.ListAllSiteSettingsRequest) (*client.ListAllSiteSettingsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListAllSiteSettingsLogic.ListAllSiteSettings")
	defer span.End()

	settings, err := l.svcCtx.Repo.SiteSettings.ListAllSiteSettings(ctx)
	if err != nil {
		return nil, err
	}

	pbSettings := make([]*client.SiteSetting, len(settings))
	for i, s := range settings {
		pbSettings[i] = convertSiteSetting(s)
	}

	return &client.ListAllSiteSettingsResponse{
		Settings: pbSettings,
	}, nil
}
