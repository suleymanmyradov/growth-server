package sitesettingslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type DeleteSiteSettingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteSiteSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSiteSettingLogic {
	return &DeleteSiteSettingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteSiteSettingLogic) DeleteSiteSetting(in *client.DeleteSiteSettingRequest) (*client.DeleteSiteSettingResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteSiteSettingLogic.DeleteSiteSetting")
	defer span.End()

	if err := l.svcCtx.Repo.SiteSettings.DeleteSiteSetting(ctx, in.Key); err != nil {
		return nil, err
	}

	return &client.DeleteSiteSettingResponse{
		Success: true,
	}, nil
}
