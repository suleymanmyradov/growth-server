package goals

import (
	"context"
	"time"

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

	rpcResp, err := l.svcCtx.GoalsRpc.ListGoals(l.ctx, &clientgoals.ListGoalsRequest{
		UserId: "",
		Page:   int32(req.Page),
		Limit:  int32(req.Limit),
	})
	if err != nil {
		return nil, err
	}

	var goals []types.Goal
	for _, g := range rpcResp.Goals {
		goals = append(goals, types.Goal{
			Id:          g.Id,
			Title:       g.Title,
			Description: g.Description,
			Category:    g.Category,
			DueDate:     formatTime(g.DueDate),
			Progress:    int(g.Progress),
			Completed:   g.Completed,
			UserId:      g.UserId,
			CreatedAt:   formatTime(g.CreatedAt),
			UpdatedAt:   formatTime(g.UpdatedAt),
		})
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

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}
