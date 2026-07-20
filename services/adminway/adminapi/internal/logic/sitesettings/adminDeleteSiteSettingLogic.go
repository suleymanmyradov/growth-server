// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package sitesettings

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientsitesettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/sitesettings"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDeleteSiteSettingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDeleteSiteSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteSiteSettingLogic {
	return &AdminDeleteSiteSettingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteSiteSettingLogic) AdminDeleteSiteSetting(req *types.DeleteSiteSettingRequest) (resp *types.EmptyResponse, err error) {
	_, err = l.svcCtx.SiteSettingsRpc.DeleteSiteSetting(l.ctx, &clientsitesettings.DeleteSiteSettingRequest{
		Key: req.Key,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
