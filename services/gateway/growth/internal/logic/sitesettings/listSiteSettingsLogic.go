// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package sitesettings

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientsitesettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/sitesettings"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSiteSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSiteSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSiteSettingsLogic {
	return &ListSiteSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSiteSettingsLogic) ListSiteSettings() (resp *types.SiteSettingsResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.SiteSettings.ListAllSiteSettings(l.ctx, &clientsitesettings.ListAllSiteSettingsRequest{})
	if err != nil {
		return nil, err
	}

	items := make([]types.SiteSettingItem, 0, len(rpcResp.Settings))
	for _, s := range rpcResp.Settings {
		items = append(items, types.SiteSettingItem{
			Key:       s.Key,
			Value:     string(s.Value),
			UpdatedAt: formatTime(s.UpdatedAt),
		})
	}

	return &types.SiteSettingsResponse{
		Data: items,
	}, nil
}

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}
