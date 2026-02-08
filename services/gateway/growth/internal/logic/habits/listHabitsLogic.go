// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package habits

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListHabitsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHabitsLogic {
	return &ListHabitsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListHabitsLogic) ListHabits(req *types.PageRequest) (resp *types.HabitsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
