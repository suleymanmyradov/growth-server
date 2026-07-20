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

type AdminGetSiteSettingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetSiteSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetSiteSettingLogic {
	return &AdminGetSiteSettingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetSiteSettingLogic) AdminGetSiteSetting(req *types.DeleteSiteSettingRequest) (resp *types.SiteSettingResponse, err error) {
	rpcResp, err := l.svcCtx.SiteSettingsRpc.GetSiteSetting(l.ctx, &clientsitesettings.GetSiteSettingRequest{
		Key: req.Key,
	})
	if err != nil {
		return nil, err
	}

	return &types.SiteSettingResponse{
		Data: mapSiteSetting(rpcResp.Setting),
	}, nil
}
