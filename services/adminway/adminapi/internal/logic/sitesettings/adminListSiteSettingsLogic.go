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

type AdminListSiteSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListSiteSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListSiteSettingsLogic {
	return &AdminListSiteSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListSiteSettingsLogic) AdminListSiteSettings() (resp *types.SiteSettingsResponse, err error) {
	rpcResp, err := l.svcCtx.SiteSettingsRpc.ListAllSiteSettings(l.ctx, &clientsitesettings.ListAllSiteSettingsRequest{})
	if err != nil {
		return nil, err
	}

	items := make([]types.SiteSettingItem, 0, len(rpcResp.Settings))
	for _, s := range rpcResp.Settings {
		items = append(items, mapSiteSetting(s))
	}

	return &types.SiteSettingsResponse{
		Data: items,
	}, nil
}
