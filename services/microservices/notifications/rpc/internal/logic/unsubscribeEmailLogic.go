package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/unsubtoken"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UnsubscribeEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnsubscribeEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnsubscribeEmailLogic {
	return &UnsubscribeEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UnsubscribeEmail disables email_notifications for the user identified by the
// HMAC token minted into List-Unsubscribe URLs by the delivery worker
// (RFC 8058 one-click unsubscribe). No principal is present — this method is
// exempt from user-JWT verification and authenticates via the token instead.
// Other preference toggles are preserved.
func (l *UnsubscribeEmailLogic) UnsubscribeEmail(in *notifications.UnsubscribeEmailRequest) (*notifications.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UnsubscribeEmailLogic.UnsubscribeEmail")
	defer span.End()

	if l.svcCtx.EmailUnsubscribeSecret == "" {
		l.Errorf("UnsubscribeEmail: unsubscribe secret is not configured")
		return nil, status.Error(codes.Internal, "unsubscribe is not available")
	}

	userID, err := unsubtoken.Verify(l.svcCtx.EmailUnsubscribeSecret, in.Token)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid unsubscribe token")
	}

	prev, err := l.svcCtx.Repo.Preferences.Get(ctx, userID)
	if err != nil {
		l.Errorf("UnsubscribeEmail: get preferences for %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "unsubscribe failed")
	}

	if _, err := l.svcCtx.Repo.Preferences.Upsert(ctx, db.UpsertNotificationPreferencesParams{
		UserID:             userID,
		EmailNotifications: false,
		PushNotifications:  prev.PushNotifications,
		HabitReminders:     prev.HabitReminders,
		GoalReminders:      prev.GoalReminders,
		StreakWarnings:     prev.StreakWarnings,
		SundayReview:       prev.SundayReview,
	}); err != nil {
		l.Errorf("UnsubscribeEmail: disable email for %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "unsubscribe failed")
	}

	l.Infof("email notifications unsubscribed via List-Unsubscribe for user %s", userID)
	return &notifications.EmptyResponse{}, nil
}
