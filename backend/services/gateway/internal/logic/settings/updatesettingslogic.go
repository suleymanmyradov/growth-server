// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package settings

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

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
	// todo: add your logic here and delete this line

	return
}
