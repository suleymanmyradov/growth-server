package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/activity"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetActivityFeedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetActivityFeedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityFeedLogic {
	return &GetActivityFeedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Activity feed
func (l *GetActivityFeedLogic) GetActivityFeed(in *activity.GetActivityFeedRequest) (*activity.GetActivityFeedResponse, error) {
	// todo: add your logic here and delete this line

	return &activity.GetActivityFeedResponse{}, nil
}
