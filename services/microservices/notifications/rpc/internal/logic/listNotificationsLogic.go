package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListNotificationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListNotificationsLogic {
	return &ListNotificationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListNotificationsLogic) ListNotifications(in *notifications.ListNotificationsRequest) (*notifications.ListNotificationsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListNotificationsLogic.ListNotifications")
	defer span.End()

	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
		if limit > 100 {
			limit = 100
		}
	}
	if in.Offset >= 0 {
		offset = in.Offset
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	var result []*notifications.Notification
	var totalCount, unreadCount int64

	if in.OnlyUnread {
		dbNotifications, err := l.svcCtx.Repo.Notifications.ListUnreadNotifications(ctx, userID, limit, offset)
		if err != nil {
			logx.WithContext(ctx).Errorf("Failed to list unread notifications: %v", err)
			return nil, status.Error(codes.Internal, "failed to list unread notifications")
		}
		for _, n := range dbNotifications {
			result = append(result, listUnreadNotificationToProto(n))
		}
		totalCount, err = l.svcCtx.Repo.Notifications.GetUnreadCount(ctx, userID)
		if err != nil {
			logx.WithContext(ctx).Errorf("Failed to count unread notifications: %v", err)
			return nil, status.Error(codes.Internal, "failed to count unread notifications")
		}
	} else {
		dbNotifications, err := l.svcCtx.Repo.Notifications.ListNotificationsForUser(ctx, userID, limit, offset)
		if err != nil {
			logx.WithContext(ctx).Errorf("Failed to list notifications: %v", err)
			return nil, status.Error(codes.Internal, "failed to list notifications")
		}
		for _, n := range dbNotifications {
			result = append(result, listNotificationToProto(n))
		}
		totalCount, err = l.svcCtx.Repo.Notifications.CountNotificationsByUser(ctx, userID)
		if err != nil {
			logx.WithContext(ctx).Errorf("Failed to count notifications: %v", err)
			return nil, status.Error(codes.Internal, "failed to count notifications")
		}
	}
	unreadCount, err = l.svcCtx.Repo.Notifications.GetUnreadCount(ctx, userID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to get unread count: %v", err)
		return nil, status.Error(codes.Internal, "failed to get unread count")
	}

	return &notifications.ListNotificationsResponse{
		Notifications: result,
		TotalCount:    int32(totalCount),
		UnreadCount:   int32(unreadCount),
	}, nil
}
