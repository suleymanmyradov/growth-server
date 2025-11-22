// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGoalsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListGoalsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGoalsLogic {
	return &ListGoalsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListGoalsLogic) ListGoals(req *types.PageRequest) (resp *types.GoalsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
