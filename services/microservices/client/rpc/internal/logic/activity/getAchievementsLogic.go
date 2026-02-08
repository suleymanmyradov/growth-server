package activitylogic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	rows, err := l.svcCtx.Repo.Activities.GetAchievements(l.ctx, userID)
	if err != nil {
		l.Logger.Errorf("Failed to get achievements: %v", err)
		return nil, err
	}

	var achievements []*client.Achievement
	for _, r := range rows {
		achievements = append(achievements, &client.Achievement{
			Id:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			IconUrl:     r.IconUrl.String,
			UnlockedAt:  toUnix(r.UnlockedAt),
		})
	}

	return &client.GetAchievementsResponse{Achievements: achievements}, nil
}

func toUnix(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case time.Time:
		return t.Unix()
	default:
		return 0
	}
}
