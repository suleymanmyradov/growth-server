package activitylogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStreaksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStreaksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStreaksLogic {
	return &GetStreaksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetStreaksLogic) GetStreaks(in *client.GetStreaksRequest) (*client.GetStreaksResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	streaks, err := l.svcCtx.Repo.Activities.GetStreaks(l.ctx, userID)
	if err != nil {
		l.Logger.Errorf("Failed to get streaks: %v", err)
		return nil, err
	}

	current := toInt32(streaks.CurrentStreak)
	longest := toInt32(streaks.LongestStreak)

	return &client.GetStreaksResponse{
		CurrentStreak: current,
		LongestStreak: longest,
	}, nil
}

func toInt32(v interface{}) int32 {
	switch val := v.(type) {
	case int64:
		return int32(val)
	case int32:
		return val
	case int:
		return int32(val)
	case float64:
		return int32(val)
	default:
		return 0
	}
}
