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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetActivityFeedLogic.GetActivityFeed")
	defer span.End()
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	activities, err := l.svcCtx.Repo.Activities.GetActivityFeed(ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("Failed to list activities: %v", err)
		return nil, status.Error(codes.Internal, "failed to list activities")
	}

	pbActivities := make([]*client.ActivityItem, len(activities))
	for i, a := range activities {
		pbActivities[i] = &client.ActivityItem{
			Id:          a.ID.String(),
			UserId:      a.UserID.String(),
			Type:        a.Type,
			Description: a.Title,
			Timestamp:   a.CreatedAt.Time.Unix(),
		}
	}

	return &client.GetActivityFeedResponse{
		Activities: pbActivities,
	}, nil
}
