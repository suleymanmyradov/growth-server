package sitesettingslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type UpsertSiteSettingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertSiteSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertSiteSettingLogic {
	return &UpsertSiteSettingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpsertSiteSettingLogic) UpsertSiteSetting(in *client.UpsertSiteSettingRequest) (*client.UpsertSiteSettingResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpsertSiteSettingLogic.UpsertSiteSetting")
	defer span.End()

	setting, err := l.svcCtx.Repo.SiteSettings.UpsertSiteSetting(ctx, in.Key, in.Value)
	if err != nil {
		return nil, err
	}

	return &client.UpsertSiteSettingResponse{
		Setting: convertSiteSetting(setting),
	}, nil
}
