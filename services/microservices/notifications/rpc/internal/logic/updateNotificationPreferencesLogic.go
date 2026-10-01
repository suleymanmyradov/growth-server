package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

	// Merge: only fields explicitly present on the request are changed; the
	// rest keep their previous values. Without proto3 optional presence an
	// omitted bool would serialize as false and silently reset every other
	// toggle (and fire false enabled->disabled transitions below).
	pref, err := l.svcCtx.Repo.Preferences.Upsert(ctx, db.UpsertNotificationPreferencesParams{
		UserID:             userID,
		EmailNotifications: pickBool(in.Preferences.EmailEnabled, prev.EmailNotifications),
		PushNotifications:  pickBool(in.Preferences.PushEnabled, prev.PushNotifications),
		HabitReminders:     pickBool(in.Preferences.HabitRemindersEnabled, prev.HabitReminders),
		GoalReminders:      pickBool(in.Preferences.GoalRemindersEnabled, prev.GoalReminders),
		StreakWarnings:     pickBool(in.Preferences.StreakWarningsEnabled, prev.StreakWarnings),
		SundayReview:       pickBool(in.Preferences.SundayReviewEnabled, prev.SundayReview),
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
		// Re-enabled: sync the flag first so a missing reminder_state row gets
		// created with server defaults, then schedule the next habit_reminder.
		if err := l.syncReminderStateHabitFlag(ctx, userID, true); err != nil {
			logx.WithContext(ctx).Errorf("Failed to sync reminder state habit flag: %v", err)
		}
		if err := l.scheduleNextHabitReminder(ctx, userID); err != nil {
			logx.WithContext(ctx).Errorf("Failed to schedule next habit reminder: %v", err)
		}
	}

	// Goal reminders: cancel pending goal_deadline reminders when disabled;
	// reschedule from the local goal_state read model when re-enabled.
	goalWasEnabled := prev.GoalReminders
	goalNowEnabled := pref.GoalReminders

	if goalWasEnabled && !goalNowEnabled {
		if _, err := l.svcCtx.Repo.Reminders.CancelPendingByType(ctx, userID, "goal_deadline"); err != nil {
			logx.WithContext(ctx).Errorf("Failed to cancel pending goal reminders: %v", err)
		}
	} else if !goalWasEnabled && goalNowEnabled {
		if err := l.scheduleNextGoalDeadlines(ctx, userID); err != nil {
			logx.WithContext(ctx).Errorf("Failed to schedule goal deadline reminders: %v", err)
		}
	}

	if prev.SundayReview != pref.SundayReview {
		if _, err := l.svcCtx.Repo.Reminders.CancelPendingByType(ctx, userID, "weekly_review"); err != nil {
			logx.WithContext(ctx).Errorf("Failed to cancel pending weekly reviews: %v", err)
		}
		if pref.SundayReview {
			if err := l.scheduleNextWeeklyReview(ctx, userID); err != nil {
				logx.WithContext(ctx).Errorf("Failed to schedule weekly review: %v", err)
			}
		}
	}
	if prev.StreakWarnings != pref.StreakWarnings {
		if _, err := l.svcCtx.Repo.Reminders.CancelPendingByType(ctx, userID, "streak_warning_scan"); err != nil {
			logx.WithContext(ctx).Errorf("Failed to cancel pending streak warnings: %v", err)
		}
		if pref.StreakWarnings {
			if err := l.scheduleNextStreakWarning(ctx, userID); err != nil {
				logx.WithContext(ctx).Errorf("Failed to schedule streak warning: %v", err)
			}
		}
	}

	return &notifications.UpdateNotificationPreferencesResponse{
		Preferences: &notifications.NotificationPreferences{
			EmailEnabled:          &pref.EmailNotifications,
			PushEnabled:           &pref.PushNotifications,
			HabitRemindersEnabled: &pref.HabitReminders,
			GoalRemindersEnabled:  &pref.GoalReminders,
			StreakWarningsEnabled: &pref.StreakWarnings,
			SundayReviewEnabled:   &pref.SundayReview,
		},
	}, nil
}

// pickBool returns *v when present, otherwise the previous stored value.
func pickBool(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

// getReminderState returns the user's reminder_state, creating the row with
// server defaults (UTC / 09:00) when none exists yet — e.g. a user who never
// saved settings toggles a preference before any onboarding/settings event.
// habitReminders is only consulted on the create path so the flag written
// matches what the caller just persisted to notification_preferences.
func (l *UpdateNotificationPreferencesLogic) getReminderState(ctx context.Context, userID uuid.UUID, habitReminders bool) (db.ReminderState, error) {
	rs, err := l.svcCtx.Repo.ReminderState.Get(ctx, userID)
	if err == nil {
		return rs, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.ReminderState{}, err
	}
	if err := l.svcCtx.Repo.ReminderState.UpsertSettings(ctx, userID, "", pgtype.Time{}, habitReminders); err != nil {
		return db.ReminderState{}, err
	}
	return l.svcCtx.Repo.ReminderState.Get(ctx, userID)
}

// scheduleNextHabitReminder enqueues the next habit_reminder based on the
// user's reminder_state (timezone + check-in time). Onboarding completion is
// not a gate — users who skipped setup still get reminders (P4/P7).
func (l *UpdateNotificationPreferencesLogic) scheduleNextHabitReminder(ctx context.Context, userID uuid.UUID) error {
	rs, err := l.getReminderState(ctx, userID, true)
	if err != nil {
		return err
	}

	now := time.Now()
	next, err := scheduler.NextOccurrence(now, rs.Timezone, rs.CheckInTime)
	if err != nil {
		return err
	}
	_, err = l.svcCtx.Repo.Reminders.Enqueue(ctx, userID, "habit_reminder", next, nil)
	return err
}

func (l *UpdateNotificationPreferencesLogic) scheduleNextWeeklyReview(ctx context.Context, userID uuid.UUID) error {
	rs, err := l.getReminderState(ctx, userID, true)
	if err != nil {
		return err
	}
	next, err := scheduler.NextWeekday(time.Now(), rs.Timezone, time.Sunday, 18, 0)
	if err != nil {
		return err
	}
	_, err = l.svcCtx.Repo.Reminders.Enqueue(ctx, userID, "weekly_review", next, nil)
	return err
}

func (l *UpdateNotificationPreferencesLogic) scheduleNextStreakWarning(ctx context.Context, userID uuid.UUID) error {
	rs, err := l.getReminderState(ctx, userID, true)
	if err != nil {
		return err
	}
	next, err := scheduler.NextDailyAt(time.Now(), rs.Timezone, 20, 0)
	if err != nil {
		return err
	}
	_, err = l.svcCtx.Repo.Reminders.Enqueue(ctx, userID, "streak_warning_scan", next, nil)
	return err
}

// scheduleNextGoalDeadlines reschedules goal_deadline reminders for all of the
// user's active goals with future deadlines, using the local goal_state read
// model. Called when the user re-enables the goalReminders preference.
func (l *UpdateNotificationPreferencesLogic) scheduleNextGoalDeadlines(ctx context.Context, userID uuid.UUID) error {
	if l.svcCtx.Repo.GoalState == nil {
		return nil
	}
	rs, err := l.getReminderState(ctx, userID, true)
	if err != nil {
		return err
	}
	now := time.Now()
	goals, err := l.svcCtx.Repo.GoalState.ListUpcomingDeadlines(ctx, userID, now)
	if err != nil {
		return fmt.Errorf("list upcoming goal deadlines: %w", err)
	}
	for _, g := range goals {
		reminderAt, rErr := scheduler.GoalDeadlineReminderTime(now, g.Deadline.Time, rs.Timezone)
		if rErr != nil {
			logx.WithContext(ctx).Errorf("compute goal deadline reminder for %s: %v", g.GoalID, rErr)
			continue
		}
		if _, eErr := l.svcCtx.Repo.Reminders.EnqueueGoalDeadline(ctx, userID, g.GoalID.String(), g.Title, reminderAt); eErr != nil {
			logx.WithContext(ctx).Errorf("enqueue goal_deadline for %s: %v", g.GoalID, eErr)
		}
	}
	return nil
}

// syncReminderStateHabitFlag updates the habit_reminders column in
// reminder_state to match the user's notification preference, preserving the
// existing timezone and check-in time. When no row exists yet, it creates one
// with server defaults — UpsertSettings treats empty timezone/NULL time as
// "not provided".
func (l *UpdateNotificationPreferencesLogic) syncReminderStateHabitFlag(ctx context.Context, userID uuid.UUID, enabled bool) error {
	rs, err := l.svcCtx.Repo.ReminderState.Get(ctx, userID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return l.svcCtx.Repo.ReminderState.UpsertSettings(ctx, userID, "", pgtype.Time{}, enabled)
	}
	return l.svcCtx.Repo.ReminderState.UpsertSettings(ctx, userID, rs.Timezone, rs.CheckInTime, enabled)
}
