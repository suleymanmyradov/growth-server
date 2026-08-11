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

type GetNotificationPreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationPreferencesLogic {
	return &GetNotificationPreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetNotificationPreferencesLogic) GetNotificationPreferences(in *notifications.GetNotificationPreferencesRequest) (*notifications.GetNotificationPreferencesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetNotificationPreferencesLogic.GetNotificationPreferences")
	defer span.End()

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	pref, err := l.svcCtx.Repo.Preferences.Get(ctx, userID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to get notification preferences: %v", err)
		return nil, status.Error(codes.Internal, "failed to get notification preferences")
	}

	return &notifications.GetNotificationPreferencesResponse{
		Preferences: &notifications.NotificationPreferences{
			EmailEnabled:          pref.EmailNotifications,
			PushEnabled:           pref.PushNotifications,
			HabitRemindersEnabled: pref.HabitReminders,
			GoalRemindersEnabled:  pref.GoalReminders,
			StreakWarningsEnabled: pref.StreakWarnings,
			SundayReviewEnabled:   pref.SundayReview,
		},
	}, nil
}
