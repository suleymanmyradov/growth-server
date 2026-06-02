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

type CreateCheckInLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateCheckInLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCheckInLogic {
	return &CreateCheckInLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateCheckInLogic) CreateCheckIn(req *types.CreateCheckInRequest) (resp *types.CreateCheckInResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if !validator.IsNotEmpty(req.HabitId) {
		return nil, status.Error(codes.InvalidArgument, "habitId is required")
	}
	if !validator.IsNotEmpty(req.Status) {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	rpcResp, err := l.svcCtx.CheckInRpc.CreateCheckIn(l.ctx, &clientcheckin.CreateCheckInRequest{
		UserId:  p.UserID,
		HabitId: req.HabitId,
		Status:  req.Status,
		Mood:    req.Mood,
		Energy:  req.Energy,
		Blocker: req.Blocker,
		Note:    req.Note,
	})
	if err != nil {
		return nil, err
	}

	return &types.CreateCheckInResponse{
		CheckIn: types.CheckIn{
			Id:        rpcResp.CheckIn.Id,
			UserId:    rpcResp.CheckIn.UserId,
			HabitId:   rpcResp.CheckIn.HabitId,
			Status:    rpcResp.CheckIn.Status,
			Mood:      rpcResp.CheckIn.Mood,
			Energy:    rpcResp.CheckIn.Energy,
			Blocker:   rpcResp.CheckIn.Blocker,
			Note:      rpcResp.CheckIn.Note,
			CreatedAt: formatTime(rpcResp.CheckIn.CreatedAt),
		},
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
		AiFeedback: rpcResp.AiFeedback,
	}, nil
}
