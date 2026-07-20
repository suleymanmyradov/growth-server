// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package sitesettings

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientsitesettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/sitesettings"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUpsertSiteSettingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpsertSiteSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpsertSiteSettingLogic {
	return &AdminUpsertSiteSettingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpsertSiteSettingLogic) AdminUpsertSiteSetting(req *types.UpsertSiteSettingRequest) (resp *types.SiteSettingResponse, err error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}
	if req.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "value is required")
	}

	// Validate that value is valid JSON before storing.
	if !json.Valid([]byte(req.Value)) {
		return nil, status.Error(codes.InvalidArgument, "value must be valid JSON")
	}

	rpcResp, err := l.svcCtx.SiteSettingsRpc.UpsertSiteSetting(l.ctx, &clientsitesettings.UpsertSiteSettingRequest{
		Key:   req.Key,
		Value: []byte(req.Value),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert site setting: %w", err)
	}

	return &types.SiteSettingResponse{
		Data: mapSiteSetting(rpcResp.Setting),
	}, nil
}
