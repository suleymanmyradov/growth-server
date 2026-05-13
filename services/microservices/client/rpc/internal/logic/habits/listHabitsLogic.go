package habitslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (l *ListHabitsLogic) ListHabits(in *client.ListHabitsRequest) (*client.ListHabitsResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (in.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	habits, err := l.svcCtx.Repo.Habits.ListHabits(l.ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("Failed to list habits: %v", err)
		return nil, err
	}

	total, err := l.svcCtx.Repo.Habits.CountHabitsByUser(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to count habits: %v", err)
		return nil, err
	}

	var pbHabits []*client.Habit
	for _, h := range habits {
		pbHabits = append(pbHabits, habitToProto(h))
	}

	return &client.ListHabitsResponse{
		Habits: pbHabits,
		Total:  int32(total),
	}, nil
}
