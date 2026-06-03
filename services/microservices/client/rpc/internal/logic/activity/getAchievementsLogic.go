package activitylogic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetAchievementsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAchievementsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAchievementsLogic {
	return &GetAchievementsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAchievementsLogic) GetAchievements(in *client.GetAchievementsRequest) (*client.GetAchievementsResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	rows, err := l.svcCtx.Repo.Activities.GetAchievements(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to get achievements: %v", err)
return nil, status.Error(codes.Internal, "failed to get achievements")
	}

	achievements := make([]*client.Achievement, 0, len(rows))
	for _, r := range rows {
		unlockedAt, ok := r.UnlockedAt.(time.Time)
		if !ok {
			l.Errorf("Invalid UnlockedAt type for achievement %s", r.ID)
			continue
		}
		iconUrl := ""
		if r.IconUrl != nil {
			iconUrl = *r.IconUrl
		}
		achievements = append(achievements, &client.Achievement{
			Id:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			IconUrl:     iconUrl,
			UnlockedAt:  ToUnix(unlockedAt),
		})
	}

	return &client.GetAchievementsResponse{Achievements: achievements}, nil
}
