// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package activity

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetActivityFeedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetActivityFeedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityFeedLogic {
	return &GetActivityFeedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetActivityFeedLogic) GetActivityFeed(req *types.PageRequest) (resp *types.ActivityResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
