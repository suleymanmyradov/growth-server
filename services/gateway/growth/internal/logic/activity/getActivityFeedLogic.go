// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package activity

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetActivityFeedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetActivityFeedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityFeedLogic {
	return &GetActivityFeedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetActivityFeedLogic) GetActivityFeed(req *types.PageRequest) (resp *types.ActivityResponse, err error) {
	limit := int32(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	offset := int32((req.Page - 1) * req.Limit)
	if offset < 0 {
		offset = 0
	}

	rpcResp, err := l.svcCtx.ActivityRpc.GetActivityFeed(l.ctx, &client.GetActivityFeedRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		l.Errorf("Failed to get activity feed: %v", err)
		return nil, err
	}

	var activities []types.Activity
	for _, a := range rpcResp.Activities {
		createdAt := ""
		if a.Timestamp > 0 {
			createdAt = time.Unix(a.Timestamp, 0).UTC().Format(time.RFC3339)
		}
		activities = append(activities, types.Activity{
			Id:          a.Id,
			ItemType:    a.Type,
			Title:       a.Description,
			Description: "",
			UserId:      a.UserId,
			CreatedAt:   createdAt,
		})
	}

	return &types.ActivityResponse{
		Data: activities,
		Page: types.PageResponse{
			Page:  req.Page,
			Limit: req.Limit,
			Total: 0,
		},
	}, nil
}
