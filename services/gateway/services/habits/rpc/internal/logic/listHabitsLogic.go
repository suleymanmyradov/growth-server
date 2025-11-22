package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListHabitsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHabitsLogic {
	return &ListHabitsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CRUD operations
func (l *ListHabitsLogic) ListHabits(in *habits.ListHabitsRequest) (*habits.ListHabitsResponse, error) {
	// todo: add your logic here and delete this line

	return &habits.ListHabitsResponse{}, nil
}
