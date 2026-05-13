// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package settings

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientsettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/settings"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSettingsLogic {
	return &UpdateSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSettingsLogic) UpdateSettings(req *types.UpdateSettingsRequest) (resp *types.SettingsResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.SettingsResponse{Data: types.Settings{}}, nil
	}

	_, err = l.svcCtx.SettingsRpc.UpdateSettings(l.ctx, &clientsettings.UpdateSettingsRequest{
		UserId: "",
		Settings: &client.UserSettings{
			Theme:               req.Theme,
			Language:            req.Language,
			Timezone:            req.Timezone,
			AccountabilityStyle: req.AccountabilityStyle,
			CheckInTime:         req.CheckInTime,
			OnboardingCompleted: req.OnboardingCompleted,
		},
	})
	if err != nil {
		return nil, err
	}

	return &types.SettingsResponse{
		Data: types.Settings{
			Theme:               req.Theme,
			Language:            req.Language,
			Timezone:            req.Timezone,
			AccountabilityStyle: req.AccountabilityStyle,
			CheckInTime:         req.CheckInTime,
			OnboardingCompleted: req.OnboardingCompleted,
			UserId:              "",
		},
	}, nil
}
