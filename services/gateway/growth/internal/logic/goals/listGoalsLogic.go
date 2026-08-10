package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, nil
	}
	l.Infof("UserID: %v", p.UserID)

	rpcResp, err := l.svcCtx.ClientRpc.Goals.ListGoals(l.ctx, &clientgoals.ListGoalsRequest{
		Page:  int32(req.Page),
		Limit: int32(req.Limit),
	})
	if err != nil {
		return nil, err
	}

	goals := make([]types.Goal, 0, len(rpcResp.Goals))
	for _, g := range rpcResp.Goals {
		goals = append(goals, rpcGoalToType(g))
	}

	totalPages := int(rpcResp.Total) / req.Limit
	if int(rpcResp.Total)%req.Limit > 0 {
		totalPages++
	}

	return &types.GoalsResponse{
		Data: goals,
		Page: types.PageResponse{
			Total:      int64(rpcResp.Total),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: totalPages,
		},
	}, nil
}
