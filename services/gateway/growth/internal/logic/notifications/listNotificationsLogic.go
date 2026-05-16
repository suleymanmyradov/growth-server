// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package notifications

import (
	"context"
	"strconv"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListNotificationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListNotificationsLogic {
	return &ListNotificationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListNotificationsLogic) ListNotifications(req *types.PageRequest) (resp *types.NotificationsResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.NotificationsResponse{Data: []types.Notification{}}, nil
	}

	rpcResp, err := l.svcCtx.NotificationsRpc.ListNotifications(l.ctx, &notificationsClient.ListNotificationsRequest{
		Limit:  int32(req.Limit),
		Offset: int32((req.Page - 1) * req.Limit),
	})
	if err != nil {
		return nil, err
	}

	notifications := make([]types.Notification, 0, len(rpcResp.Notifications))
	for _, n := range rpcResp.Notifications {
		notifications = append(notifications, types.Notification{
			Id:        n.Id,
			Title:     n.Title,
			Message:   n.Message,
			ItemType:  n.Type,
			Read:      n.Read,
			UserId:    n.UserId,
			CreatedAt: strconv.FormatInt(n.CreatedAt, 10),
		})
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}

	totalPages := int(rpcResp.TotalCount) / req.Limit
	if int(rpcResp.TotalCount)%req.Limit > 0 {
		totalPages++
	}

	return &types.NotificationsResponse{
		Data: notifications,
		Page: types.PageResponse{
			Total:      int64(rpcResp.TotalCount),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: totalPages,
		},
	}, nil
}
