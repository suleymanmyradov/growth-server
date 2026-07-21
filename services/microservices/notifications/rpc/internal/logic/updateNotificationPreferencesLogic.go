package logic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/scheduler"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateNotificationPreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateNotificationPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateNotificationPreferencesLogic {
	return &UpdateNotificationPreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateNotificationPreferencesLogic) UpdateNotificationPreferences(in *notifications.UpdateNotificationPreferencesRequest) (*notifications.UpdateNotificationPreferencesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateNotificationPreferencesLogic.UpdateNotificationPreferences")
	defer span.End()

	if in.Preferences == nil {
		return nil, status.Error(codes.InvalidArgument, "preferences are required")
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

	// Fetch the previous preferences to detect transitions (enabled -> disabled
	// and vice versa) so we can cancel or reschedule reminders accordingly.
	prev, err := l.svcCtx.Repo.Preferences.Get(ctx, userID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to get previous preferences: %v", err)
		return nil, status.Error(codes.Internal, "failed to get previous preferences")
	}

	pref, err := l.svcCtx.Repo.Preferences.Upsert(ctx, db.UpsertNotificationPreferencesParams{
		UserID:             userID,
		EmailNotifications: in.Preferences.EmailEnabled,
		PushNotifications:  in.Preferences.PushEnabled,
		HabitReminders:     in.Preferences.HabitRemindersEnabled,
		GoalReminders:      in.Preferences.GoalRemindersEnabled,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to upsert notification preferences: %v", err)
		return nil, status.Error(codes.Internal, "failed to update notification preferences")
	}

	// Apply reminder side-effects for habit reminders.
	habitWasEnabled := prev.HabitReminders
	habitNowEnabled := pref.HabitReminders

	if habitWasEnabled && !habitNowEnabled {
		// Disabled: cancel all pending habit_reminder reminders.
		if _, err := l.svcCtx.Repo.Reminders.CancelPendingByType(ctx, userID, "habit_reminder"); err != nil {
			logx.WithContext(ctx).Errorf("Failed to cancel pending habit reminders: %v", err)
		}
		// Sync reminder_state so scheduleRemindersFromState respects the change.
		if err := l.syncReminderStateHabitFlag(ctx, userID, false); err != nil {
			logx.WithContext(ctx).Errorf("Failed to sync reminder state habit flag: %v", err)
		}
	} else if !habitWasEnabled && habitNowEnabled {
		// Re-enabled: schedule the next habit_reminder from reminder_state.
		if err := l.scheduleNextHabitReminder(ctx, userID); err != nil {
			logx.WithContext(ctx).Errorf("Failed to schedule next habit reminder: %v", err)
		}
		if err := l.syncReminderStateHabitFlag(ctx, userID, true); err != nil {
			logx.WithContext(ctx).Errorf("Failed to sync reminder state habit flag: %v", err)
		}
	}

	// Goal reminders: cancel pending goal_deadline reminders when disabled.
	// Rescheduling goal-deadline reminders is not yet implemented (no goal
	// deadline producer exists); only cancellation is applied.
	if prev.GoalReminders && !pref.GoalReminders {
		if _, err := l.svcCtx.Repo.Reminders.CancelPendingByType(ctx, userID, "goal_deadline"); err != nil {
			logx.WithContext(ctx).Errorf("Failed to cancel pending goal reminders: %v", err)
		}
	}

	return &notifications.UpdateNotificationPreferencesResponse{
		Preferences: &notifications.NotificationPreferences{
			EmailEnabled:          pref.EmailNotifications,
			PushEnabled:           pref.PushNotifications,
			HabitRemindersEnabled: pref.HabitReminders,
			GoalRemindersEnabled:  pref.GoalReminders,
		},
	}, nil
}

// scheduleNextHabitReminder enqueues the next habit_reminder based on the
// user's reminder_state (timezone + check-in time). It is a no-op if the user
// has not completed onboarding or has no reminder state yet.
func (l *UpdateNotificationPreferencesLogic) scheduleNextHabitReminder(ctx context.Context, userID uuid.UUID) error {
	rs, err := l.svcCtx.Repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return err
	}
	if !rs.OnboardingCompleted {
		return nil
	}

	now := time.Now()
	next, err := scheduler.NextOccurrence(now, rs.Timezone, rs.CheckInTime)
	if err != nil {
		return err
	}
	_, err = l.svcCtx.Repo.Reminders.Enqueue(ctx, userID, "habit_reminder", next, nil)
	return err
}

// syncReminderStateHabitFlag updates the habit_reminders column in
// reminder_state to match the user's notification preference, preserving the
// existing timezone and check-in time.
func (l *UpdateNotificationPreferencesLogic) syncReminderStateHabitFlag(ctx context.Context, userID uuid.UUID, enabled bool) error {
	rs, err := l.svcCtx.Repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return err
	}
	return l.svcCtx.Repo.ReminderState.UpsertSettings(ctx, userID, rs.Timezone, rs.CheckInTime, enabled)
}
