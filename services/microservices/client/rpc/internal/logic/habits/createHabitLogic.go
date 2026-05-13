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

type CreateHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateHabitLogic {
	return &CreateHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateHabitLogic) CreateHabit(in *client.CreateHabitRequest) (*client.CreateHabitResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	params := protoToHabitParams(in.Name, in.Description, in.Category, userID)
	habit, err := l.svcCtx.Repo.Habits.CreateHabit(l.ctx, params)
	if err != nil {
		l.Errorf("Failed to create habit: %v", err)
		return nil, err
	}

	return &client.CreateHabitResponse{
		Habit: habitToProto(habit),
	}, nil
}
