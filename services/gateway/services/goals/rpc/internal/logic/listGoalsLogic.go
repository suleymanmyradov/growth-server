package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGoalsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListGoalsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGoalsLogic {
	return &ListGoalsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CRUD operations
func (l *ListGoalsLogic) ListGoals(in *goals.ListGoalsRequest) (*goals.ListGoalsResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.ListGoalsResponse{}, nil
}
