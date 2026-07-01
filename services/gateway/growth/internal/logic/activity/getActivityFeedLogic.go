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

	activities := make([]types.Activity, 0, len(rpcResp.Activities))
	for _, a := range rpcResp.Activities {
		createdAt := ""
		if a.Timestamp > 0 {
			createdAt = time.Unix(a.Timestamp, 0).UTC().Format(time.RFC3339)
		}
		activities = append(activities, types.Activity{
			Id:          a.Id,
			ItemType:    a.Type,
			Title:       a.Title,
			Description: a.Description,
			UserId:      a.UserId,
			CreatedAt:   createdAt,
		})
	}

	total := int64(len(activities))
	// If we got a full page, there may be more items — report a total that
	// signals pagination is available. The RPC doesn't return a count, so we
	// approximate: if the page is full, assume at least one more page exists.
	if int32(len(activities)) >= limit {
		total = int64(req.Page+1) * int64(req.Limit)
	}

	return &types.ActivityResponse{
		Data: activities,
		Page: types.PageResponse{
			Page:  req.Page,
			Limit: req.Limit,
			Total: total,
		},
	}, nil
}
