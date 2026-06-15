package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateNotificationLogic {
	return &CreateNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateNotificationLogic) CreateNotification(in *notifications.CreateNotificationRequest) (*notifications.CreateNotificationResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.Type == "" {
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}
	if in.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	if in.Message == "" {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}

	var userID uuid.UUID
	var err error

	if in.UserId != "" {
		userID, err = uuid.Parse(in.UserId)
		if err != nil {
			l.Errorf("Invalid user ID in request: %v", err)
			return nil, status.Error(codes.InvalidArgument, "invalid user ID")
		}
	} else {
		p, ok := principal.PrincipalFrom(l.ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing principal and no userId provided")
		}
		userID, err = uuid.Parse(p.UserID)
		if err != nil {
			l.Errorf("Invalid user ID from principal: %v", err)
			return nil, status.Error(codes.Internal, "invalid user ID")
		}
	}

	notification, err := l.svcCtx.Repo.Notifications.CreateNotification(l.ctx, in.Title, in.Message, in.Type, userID)
	if err != nil {
		l.Errorf("Failed to create notification: %v", err)
		return nil, status.Error(codes.Internal, "failed to create notification")
	}

	return &notifications.CreateNotificationResponse{
		NotificationId: notification.ID.String(),
	}, nil
}
