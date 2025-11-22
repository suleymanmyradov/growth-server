package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/activity"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAchievementsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAchievementsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAchievementsLogic {
	return &GetAchievementsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAchievementsLogic) GetAchievements(in *activity.GetAchievementsRequest) (*activity.GetAchievementsResponse, error) {
	// todo: add your logic here and delete this line

	return &activity.GetAchievementsResponse{}, nil
}
