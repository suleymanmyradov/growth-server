// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package checkin

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientcheckin "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteCheckInLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteCheckInLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCheckInLogic {
	return &DeleteCheckInLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteCheckInLogic) DeleteCheckIn(req *types.DeleteCheckInRequest) (resp *types.DeleteCheckInResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if !validator.IsNotEmpty(req.HabitId) {
		return nil, status.Error(codes.InvalidArgument, "habitId is required")
	}

	rpcResp, err := l.svcCtx.ClientRpc.CheckInService.DeleteCheckIn(l.ctx, &clientcheckin.DeleteCheckInRequest{
		UserId:  p.UserID,
		HabitId: req.HabitId,
	})
	if err != nil {
		return nil, err
	}

	return &types.DeleteCheckInResponse{
		Habit: types.Habit{
			Id:          rpcResp.Habit.Id,
			Name:        rpcResp.Habit.Name,
			Description: rpcResp.Habit.Description,
			Streak:      int(rpcResp.Habit.Streak),
			Completed:   rpcResp.Habit.Completed,
			Category:    rpcResp.Habit.Category,
			UserId:      rpcResp.Habit.UserId,
			CreatedAt:   formatTime(rpcResp.Habit.CreatedAt),
			UpdatedAt:   formatTime(rpcResp.Habit.UpdatedAt),
		},
	}, nil
}
