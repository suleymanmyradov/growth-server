package sitesettingslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type GetSiteSettingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSiteSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSiteSettingLogic {
	return &GetSiteSettingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSiteSettingLogic) GetSiteSetting(in *client.GetSiteSettingRequest) (*client.GetSiteSettingResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetSiteSettingLogic.GetSiteSetting")
	defer span.End()

	setting, err := l.svcCtx.Repo.SiteSettings.GetSiteSetting(ctx, in.Key)
	if err != nil {
		return nil, err
	}

	return &client.GetSiteSettingResponse{
		Setting: convertSiteSetting(setting),
	}, nil
}
