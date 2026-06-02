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

type ResetTodayHabitsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetTodayHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetTodayHabitsLogic {
	return &ResetTodayHabitsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetTodayHabitsLogic) ResetTodayHabits(in *client.ResetTodayHabitsRequest) (*client.ResetTodayHabitsResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	if l.svcCtx.Authz != nil {
		if err := l.svcCtx.Authz.CheckPrincipal(l.ctx); err != nil {
			return nil, err
		}
	}

	count, err := l.svcCtx.Repo.Habits.ResetTodayHabits(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to reset today habits: %v", err)
return nil, status.Error(codes.Internal, "failed to reset today habits")
	}

	return &client.ResetTodayHabitsResponse{
		ResetCount: int32(count),
	}, nil
}
