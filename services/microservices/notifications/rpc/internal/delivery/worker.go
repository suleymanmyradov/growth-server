package delivery

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	deliveryBatchSize    = 100
	deliveryLeaseMinutes = 5
	deliveryMaxAttempts  = 5
)

type Worker struct {
	repo            *repository.Repository
	push            *Sender
	email           email.Sender
	frontendBaseURL string
	interval        time.Duration
}

func NewWorker(repo *repository.Repository, push *Sender, emailSender email.Sender, frontendBaseURL string) *Worker {
	return &Worker{repo: repo, push: push, email: emailSender, frontendBaseURL: strings.TrimRight(frontendBaseURL, "/"), interval: 5 * time.Second}
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
		destination := Destination("")
		if n.Destination != nil {
			destination = Destination(*n.Destination)
		}
		resourceID := uuid.Nil
		if n.ResourceID.Valid {
			resourceID = n.ResourceID.UUID
		}
		payload, err := NewPayload(n.Title, n.Message, n.ID, destination, resourceID)
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
			IdempotencyKey: item.ID.String(),
		}); err != nil {
			return err
		}
		return w.repo.Deliveries.MarkSent(ctx, item.ID, nil)
	default:
		return w.repo.Deliveries.MarkFailed(ctx, item.ID, "invalid_channel", "unsupported delivery channel")
	}
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
	const markup = `<div style="font-family:system-ui,sans-serif;max-width:560px;margin:auto;color:#17202a"><p>Hi {{.Name}},</p><h1 style="font-size:24px">{{.Title}}</h1><p style="line-height:1.6">{{.Message}}</p><p><a href="{{.ActionURL}}" style="display:inline-block;background:#0d9488;color:white;text-decoration:none;padding:12px 20px;border-radius:8px">Open Growth</a></p></div>`
	t, err := template.New("notification").Parse(markup)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, map[string]string{"Name": name, "Title": n.Title, "Message": n.Message, "ActionURL": actionURL}); err != nil {
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
