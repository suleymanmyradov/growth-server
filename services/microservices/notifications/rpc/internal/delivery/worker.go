package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/unsubtoken"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	deliveryBatchSize    = 100
	deliveryLeaseMinutes = 5
	deliveryMaxAttempts  = 5

	// P7 product policy: quiet hours 21:00–08:00 in the user's local timezone
	// defer pushes (never drop them), and at most maxDailyPushes pushes reach
	// a device per user per local day. Neither affects the in-app feed — the
	// notification row is always created.
	quietHoursStartHour = 21
	quietHoursEndHour   = 8
	maxDailyPushes      = 5
)

type Worker struct {
	repo              *repository.Repository
	push              *Sender
	email             email.Sender
	frontendBaseURL   string
	apiBaseURL        string
	unsubscribeSecret string
	interval          time.Duration
}

func NewWorker(repo *repository.Repository, push *Sender, emailSender email.Sender, frontendBaseURL, apiBaseURL, unsubscribeSecret string) *Worker {
	return &Worker{
		repo:              repo,
		push:              push,
		email:             emailSender,
		frontendBaseURL:   strings.TrimRight(frontendBaseURL, "/"),
		apiBaseURL:        strings.TrimRight(apiBaseURL, "/"),
		unsubscribeSecret: unsubscribeSecret,
		interval:          5 * time.Second,
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	if _, err := w.repo.Deliveries.ReleaseStale(ctx, deliveryLeaseMinutes); err != nil {
		logx.WithContext(ctx).Errorf("release stale notification deliveries: %v", err)
	}
	deliveries, err := w.repo.Deliveries.Claim(ctx, deliveryBatchSize)
	if err != nil {
		logx.WithContext(ctx).Errorf("claim notification deliveries: %v", err)
		return
	}
	for _, item := range deliveries {
		if err := w.deliver(ctx, item); err != nil {
			w.handleFailure(ctx, item, err)
		}
	}
}

func (w *Worker) deliver(ctx context.Context, item db.NotificationDelivery) error {
	n, err := w.repo.Notifications.GetNotificationByID(ctx, item.NotificationID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "notification_missing", "notification no longer exists")
		}
		return fmt.Errorf("get notification: %w", err)
	}
	pref, err := w.repo.Preferences.Get(ctx, item.UserID)
	if err != nil {
		return fmt.Errorf("get notification preferences: %w", err)
	}
	if n.Type == "weekly_review" && !pref.SundayReview {
		return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "weekly_review_disabled", "weekly review notifications disabled")
	}
	if n.Type == "streak_warning" && !pref.StreakWarnings {
		return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "streak_warnings_disabled", "streak warnings disabled")
	}

	switch item.Channel {
	case "push":
		if !pref.PushNotifications {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "push_disabled", "push notifications disabled")
		}
		if w.push == nil {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "push_unconfigured", "push provider unavailable")
		}

		// User-local time drives quiet hours and the daily cap. Users without
		// a reminder_state row fall back to UTC.
		loc := time.UTC
		if w.repo.ReminderState != nil {
			if rs, rsErr := w.repo.ReminderState.Get(ctx, item.UserID); rsErr == nil && rs.Timezone != "" {
				if l, lErr := time.LoadLocation(rs.Timezone); lErr == nil {
					loc = l
				}
			}
		}

		// Quiet hours (21:00–08:00 local): defer to 08:00 — do not drop and do
		// not consume a send attempt.
		if wakeAt, quiet := quietHoursDeferral(time.Now(), loc); quiet {
			return w.repo.Deliveries.Defer(ctx, item.ID, wakeAt, "quiet_hours", "deferred to 08:00 local (quiet hours)")
		}

		// Daily push cap: at most maxDailyPushes pushes per user per local
		// day. Excess pushes are suppressed — the feed row already exists.
		sentToday, err := w.repo.Deliveries.CountPushesSentOnDate(ctx, item.UserID, time.Now().In(loc), loc.String())
		if err != nil {
			return fmt.Errorf("count pushes sent today: %w", err)
		}
		if sentToday >= maxDailyPushes {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "daily_cap", "daily push cap reached")
		}

		destination := Destination("")
		if n.Destination != nil {
			destination = Destination(*n.Destination)
		}
		resourceID := uuid.Nil
		if n.ResourceID.Valid {
			resourceID = n.ResourceID.UUID
		}
		payload, err := NewPayload(n.Title, pushBody(n), n.ID, destination, resourceID)
		if err != nil {
			return w.repo.Deliveries.MarkFailed(ctx, item.ID, "invalid_payload", err.Error())
		}
		count, err := w.push.Send(ctx, item.UserID, payload)
		if err != nil {
			return err
		}
		if count == 0 {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "no_active_devices", "no active push devices")
		}
		return w.repo.Deliveries.MarkSent(ctx, item.ID, nil)
	case "email":
		if !pref.EmailNotifications {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "email_disabled", "email notifications disabled")
		}
		if w.email == nil {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "email_unconfigured", "email provider unavailable")
		}
		recipient, err := w.repo.Recipients.Get(ctx, item.UserID)
		if err != nil {
			if err == pgx.ErrNoRows {
				return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "recipient_missing", "email recipient unavailable")
			}
			return fmt.Errorf("get notification recipient: %w", err)
		}
		if !recipient.EmailVerified || recipient.Email == "" {
			return w.repo.Deliveries.MarkSuppressed(ctx, item.ID, "email_unverified", "verified email unavailable")
		}
		html, err := w.renderEmail(n, recipient.Name)
		if err != nil {
			return w.repo.Deliveries.MarkFailed(ctx, item.ID, "template_error", err.Error())
		}
		if err := w.email.Send(ctx, email.Email{
			To:             []string{recipient.Email},
			Subject:        n.Title,
			HTML:           html,
			Headers:        w.unsubscribeHeaders(item.UserID),
			IdempotencyKey: item.ID.String(),
		}); err != nil {
			return err
		}
		return w.repo.Deliveries.MarkSent(ctx, item.ID, nil)
	default:
		return w.repo.Deliveries.MarkFailed(ctx, item.ID, "invalid_channel", "unsupported delivery channel")
	}
}

// quietHoursDeferral reports whether `now` falls inside the user's local quiet
// hours (21:00–08:00) and, if so, the next 08:00 local the push should fire.
func quietHoursDeferral(now time.Time, loc *time.Location) (time.Time, bool) {
	local := now.In(loc)
	hour := local.Hour()
	if hour < quietHoursStartHour && hour >= quietHoursEndHour {
		return time.Time{}, false
	}
	wake := time.Date(local.Year(), local.Month(), local.Day(), quietHoursEndHour, 0, 0, 0, loc)
	if hour >= quietHoursStartHour {
		wake = wake.Add(24 * time.Hour)
	}
	return wake, true
}

// pushBody returns the lock-screen push text for a notification. AI coach
// feedback uses a generic body — the full AI-generated text stays in the app,
// never on the lock screen (P7). metadata["pushMessage"] wins when present so
// producers can override the body without a code change.
func pushBody(n db.GetNotificationRow) string {
	if n.Type == "ai_feedback" {
		if raw := pushMessageOverride(n.Metadata); raw != "" {
			return raw
		}
		return "Your coach has new feedback for you."
	}
	return n.Message
}

func pushMessageOverride(metadata []byte) string {
	if len(metadata) == 0 {
		return ""
	}
	var meta struct {
		PushMessage string `json:"pushMessage"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return ""
	}
	return meta.PushMessage
}

func (w *Worker) handleFailure(ctx context.Context, item db.NotificationDelivery, err error) {
	message := err.Error()
	if item.AttemptCount >= deliveryMaxAttempts {
		if markErr := w.repo.Deliveries.MarkFailed(ctx, item.ID, "delivery_failed", message); markErr != nil {
			logx.WithContext(ctx).Errorf("mark notification delivery failed: %v", markErr)
		}
		return
	}
	backoff := time.Duration(1<<min(item.AttemptCount, 6)) * time.Minute
	if retryErr := w.repo.Deliveries.Retry(ctx, item.ID, time.Now().Add(backoff), "delivery_retry", message); retryErr != nil {
		logx.WithContext(ctx).Errorf("retry notification delivery: %v", retryErr)
	}
}

// unsubscribeHeaders returns the RFC 8058 one-click unsubscribe headers:
// List-Unsubscribe carries the token-authenticated endpoint and
// List-Unsubscribe-Post tells mail clients (Gmail, Yahoo, Apple Mail) they may
// unsubscribe via a bare POST. Nil when email is not fully configured.
func (w *Worker) unsubscribeHeaders(userID uuid.UUID) map[string]string {
	if w.apiBaseURL == "" || w.unsubscribeSecret == "" {
		return nil
	}
	return map[string]string{
		"List-Unsubscribe":      "<" + w.unsubscribeURL(userID) + ">",
		"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
	}
}

// unsubscribeURL is the public gateway endpoint for one-click unsubscribe.
// GET intentionally does not unsubscribe (link scanners prefetch GETs); only
// the POST sent by mail clients mutates preferences.
func (w *Worker) unsubscribeURL(userID uuid.UUID) string {
	return w.apiBaseURL + "/api/v1/notifications/email-unsubscribe?token=" + unsubtoken.Sign(w.unsubscribeSecret, userID)
}

func (w *Worker) renderEmail(n db.GetNotificationRow, name string) (string, error) {
	actionURL := w.frontendBaseURL
	switch valueOrEmpty(n.Destination) {
	case string(DestinationWeeklyReview):
		actionURL += "/progress"
	case string(DestinationActivity), string(DestinationHabitDetail):
		actionURL += "/progress"
	default:
		actionURL += "/"
	}
	const markup = `<div style="font-family:system-ui,sans-serif;max-width:560px;margin:auto;color:#17202a"><p>Hi {{.Name}},</p><h1 style="font-size:24px">{{.Title}}</h1><p style="line-height:1.6">{{.Message}}</p><p><a href="{{.ActionURL}}" style="display:inline-block;background:#0d9488;color:white;text-decoration:none;padding:12px 20px;border-radius:8px">Open Evolella</a></p><hr style="border:none;border-top:1px solid #e5e7eb;margin-top:32px"><p style="font-size:12px;color:#6b7280">You received this because email notifications are enabled on your account. <a href="{{.SettingsURL}}" style="color:#6b7280">Unsubscribe</a></p></div>`
	t, err := template.New("notification").Parse(markup)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, map[string]string{
		"Name":        name,
		"Title":       n.Title,
		"Message":     n.Message,
		"ActionURL":   actionURL,
		"SettingsURL": w.frontendBaseURL + "/me",
	}); err != nil {
		return "", err
	}
	return out.String(), nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
