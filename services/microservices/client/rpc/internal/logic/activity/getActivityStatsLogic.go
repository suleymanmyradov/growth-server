package activitylogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetActivityStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetActivityStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityStatsLogic {
	return &GetActivityStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetActivityStatsLogic) GetActivityStats(in *client.GetActivityStatsRequest) (*client.GetActivityStatsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetActivityStatsLogic.GetActivityStats")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	stats, err := l.svcCtx.Repo.Activities.GetActivityStats(ctx, userID)
	if err != nil {
		l.Errorf("Failed to get activity stats: %v", err)
return nil, status.Error(codes.Internal, "failed to get activity stats")
	}

	activityCounts := map[string]int32{
		"habit_completed": int32(stats.HabitCompleted),
		"goal_created":    int32(stats.GoalCreated),
		"goal_completed":  int32(stats.GoalCompleted),
		"article_saved":   int32(stats.ArticleSaved),
	}

	return &client.GetActivityStatsResponse{
		TotalActivities: int32(stats.TotalActivities),
		ActivityCounts:  activityCounts,
	}, nil
}
