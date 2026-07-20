package sitesettingslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ListSiteSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSiteSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSiteSettingsLogic {
	return &ListSiteSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSiteSettingsLogic) ListSiteSettings(in *client.ListSiteSettingsRequest) (*client.ListSiteSettingsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListSiteSettingsLogic.ListSiteSettings")
	defer span.End()

	settings, err := l.svcCtx.Repo.SiteSettings.ListSiteSettings(ctx, in.Keys)
	if err != nil {
		return nil, err
	}

	pbSettings := make([]*client.SiteSetting, len(settings))
	for i, s := range settings {
		pbSettings[i] = convertSiteSetting(s)
	}

	return &client.ListSiteSettingsResponse{
		Settings: pbSettings,
	}, nil
}
