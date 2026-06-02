package habits

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"


	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
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
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	rpcResp, err := l.svcCtx.HabitsRpc.ListHabits(l.ctx, &clienthabits.ListHabitsRequest{
		UserId: "",
		Page:   int32(req.Page),
		Limit:  int32(req.Limit),
	})
	if err != nil {
		return nil, err
	}

	var habits []types.Habit
	for _, h := range rpcResp.Habits {
		habits = append(habits, types.Habit{
			Id:          h.Id,
			Name:        h.Name,
			Description: h.Description,
			Streak:      int(h.Streak),
			Completed:   h.Completed,
			Category:    h.Category,
			UserId:      h.UserId,
			CreatedAt:   formatTime(h.CreatedAt),
			UpdatedAt:   formatTime(h.UpdatedAt),
		})
	}

	totalPages := int(rpcResp.Total) / req.Limit
	if int(rpcResp.Total)%req.Limit > 0 {
		totalPages++
	}

	return &types.HabitsResponse{
		Data: habits,
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
