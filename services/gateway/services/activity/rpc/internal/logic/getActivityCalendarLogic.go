package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/activity"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetActivityCalendarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetActivityCalendarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityCalendarLogic {
	return &GetActivityCalendarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetActivityCalendarLogic) GetActivityCalendar(in *activity.GetActivityCalendarRequest) (*activity.GetActivityCalendarResponse, error) {
	// todo: add your logic here and delete this line

	return &activity.GetActivityCalendarResponse{}, nil
}
