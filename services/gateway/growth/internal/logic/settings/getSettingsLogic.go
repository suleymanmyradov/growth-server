// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package settings

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientsettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/settings"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSettingsLogic {
	return &GetSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSettingsLogic) GetSettings() (resp *types.SettingsResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.SettingsResponse{Data: types.Settings{}}, nil
	}

	rpcResp, err := l.svcCtx.SettingsRpc.GetSettings(l.ctx, &clientsettings.GetSettingsRequest{
		UserId: "",
	})
	if err != nil {
		return nil, err
	}

	return &types.SettingsResponse{
		Data: types.Settings{
			Id:                  rpcResp.Settings.UserId,
			Theme:               rpcResp.Settings.Theme,
			Language:            rpcResp.Settings.Language,
			Timezone:            rpcResp.Settings.Timezone,
			AccountabilityStyle: rpcResp.Settings.AccountabilityStyle,
			CheckInTime:         rpcResp.Settings.CheckInTime,
			OnboardingCompleted: rpcResp.Settings.OnboardingCompleted,
			UserId:              rpcResp.Settings.UserId,
		},
	}, nil
}
