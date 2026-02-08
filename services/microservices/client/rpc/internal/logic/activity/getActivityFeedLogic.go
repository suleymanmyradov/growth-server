package activitylogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetActivityFeedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetActivityFeedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityFeedLogic {
	return &GetActivityFeedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetActivityFeedLogic) GetActivityFeed(in *client.GetActivityFeedRequest) (*client.GetActivityFeedResponse, error) {
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	activities, err := l.svcCtx.Repo.Activities.GetActivityFeed(l.ctx, userID, limit, offset)
	if err != nil {
		l.Logger.Errorf("Failed to list activities: %v", err)
		return nil, err
	}

	var pbActivities []*client.ActivityItem
	for _, a := range activities {
		pbActivities = append(pbActivities, &client.ActivityItem{
			Id:          a.ID.String(),
			UserId:      a.UserID.String(),
			Type:        a.ItemType,
			Description: a.Title,
			Timestamp:   a.CreatedAt.Time.Unix(),
		})
	}

	return &client.GetActivityFeedResponse{
		Activities: pbActivities,
	}, nil
}
