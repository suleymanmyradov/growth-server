// Package delivery sends push notifications to users' registered devices via
// the Expo Push service. It is the bridge between the notifications service's
// in-app notification rows and the out-of-band push channel.
//
// Flow (see docs/push-notifications-design.md):
//  1. The notifications service creates an in-app notification row.
//  2. It calls Sender.Send(ctx, userID, payload) to also deliver a push.
//  3. The sender looks up the user's active devices (DevicesRepo), builds Expo
//     PushMessages (one per device, carrying the notification id + deep-link
//     data), and calls the Expo client.
//  4. Tickets are persisted to push_tickets for the receipt worker, which
//     checks receipts asynchronously and disables stale tokens via
//     DevicesRepo.DisableDeviceByToken when Expo reports DeviceNotRegistered.
//
// When Expo is disabled in config, Send is a no-op that logs and returns nil,
// so dev environments without push can run the service.
package delivery

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/notifications/expo"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// PushDataVersion is the schema version for the Data payload delivered to the
// app. The mobile app must check this version before interpreting any field.
// Bump this when the Data shape changes in a backwards-incompatible way.
const PushDataVersion = 1

// Destination is a validated internal route identifier. The mobile app maps
// this to a typed Expo Router route. It must never be an arbitrary URL.
type Destination string

// Validated deep-link destinations. Add new destinations here and validate in
// NewPayload — the app's router only accepts these values.
const (
	DestinationHabitDetail   Destination = "habit-detail"
	DestinationGoalDetail    Destination = "goal-detail"
	DestinationArticleDetail Destination = "article-detail"
	DestinationConversation  Destination = "conversation"
	DestinationWeeklyReview  Destination = "weekly-review"
	DestinationActivity      Destination = "activity"
	DestinationNotifications Destination = "notifications"
)

var validDestinations = map[Destination]bool{
	DestinationHabitDetail:   true,
	DestinationGoalDetail:    true,
	DestinationArticleDetail: true,
	DestinationConversation:  true,
	DestinationWeeklyReview:  true,
	DestinationActivity:      true,
	DestinationNotifications: true,
}

// Payload is the push notification content. Data is delivered to the app on tap
// and is the primary mechanism for deep-link routing (e.g. opening a specific
// habit's check-in screen).
//
// Destination and ResourceID are validated: Destination must be one of the
// constants above, and ResourceID (when set) must be a valid UUID. The Data
// map is built by the sender from these validated fields — callers must not
// pass arbitrary data. This prevents arbitrary URLs from reaching the router.
type Payload struct {
	Title    string
	Body     string
	Sound    string // "default" or a custom sound file name; empty = silent
	Priority string // "default" | "normal" | "high"

	// NotificationID is the in-app notification row ID, included in Data so the
	// app can mark the notification read on tap.
	NotificationID uuid.UUID

	// Destination is the validated internal route identifier for deep-linking.
	// Empty means no deep-link (the app opens to the default screen).
	Destination Destination

	// ResourceID is the optional resource UUID for the destination (e.g. habit
	// ID for habit-detail). Empty UUID means no resource.
	ResourceID uuid.UUID
}

// NewPayload validates and constructs a Payload. Returns an error if the
// destination is invalid or the title is empty.
func NewPayload(title, body string, notificationID uuid.UUID, dest Destination, resourceID uuid.UUID) (Payload, error) {
	if title == "" {
		return Payload{}, fmt.Errorf("push payload: title is required")
	}
	if dest != "" && !validDestinations[dest] {
		return Payload{}, fmt.Errorf("push payload: invalid destination %q", dest)
	}
	return Payload{
		Title:          title,
		Body:           body,
		Sound:          "default",
		Priority:       "default",
		NotificationID: notificationID,
		Destination:    dest,
		ResourceID:     resourceID,
	}, nil
}

// dataMap builds the validated Data map for the Expo push message. The mobile
// app reads version first, then destination + resourceId + notificationId.
// No arbitrary URLs or unvalidated strings are included.
func (p Payload) dataMap() map[string]any {
	data := map[string]any{
		"version":        PushDataVersion,
		"notificationId": p.NotificationID.String(),
	}
	if p.Destination != "" {
		data["destination"] = string(p.Destination)
	}
	if p.ResourceID != uuid.Nil {
		data["resourceId"] = p.ResourceID.String()
	}
	return data
}

// Sender pushes notifications to users' devices via Expo.
type Sender struct {
	devices *repository.DevicesRepo
	tickets *repository.PushTicketsRepo
	expo    *expo.Client
	enabled bool
}

// NewSender returns a push delivery sender. When enabled is false, Send is a
// no-op (used in dev environments without push configured). tickets may be nil
// if ticket persistence is not needed (e.g. in tests).
func NewSender(devices *repository.DevicesRepo, tickets *repository.PushTicketsRepo, expoClient *expo.Client, enabled bool) *Sender {
	return &Sender{devices: devices, tickets: tickets, expo: expoClient, enabled: enabled}
}

// Send delivers a push notification to all of the user's active devices. It is
// best-effort: a failure to push does NOT fail the caller's operation (the
// in-app notification row is the source of truth). Errors are logged.
//
// Returns the number of devices the push was attempted on (0 if the user has no
// active devices or push is disabled). A nil error means either no devices
// (nothing to do) or Expo accepted all messages (delivery is async from here).
//
// Successful tickets are persisted to push_tickets so the receipt worker can
// check delivery status asynchronously and disable stale tokens.
func (s *Sender) Send(ctx context.Context, userID uuid.UUID, payload Payload) (int, error) {
	if !s.enabled || s.expo == nil {
		logx.WithContext(ctx).Debugf("push delivery disabled, skipping for user %s", userID)
		return 0, nil
	}
	if payload.Title == "" {
		return 0, fmt.Errorf("push delivery: title is required")
	}

	devices, err := s.devices.ListActiveDevicesByUser(ctx, userID)
	if err != nil {
		logx.WithContext(ctx).Errorf("push delivery: list devices for user %s failed: %v", userID, err)
		return 0, fmt.Errorf("list devices: %w", err)
	}
	if len(devices) == 0 {
		return 0, nil
	}

	data := payload.dataMap()
	msgs := make([]expo.PushMessage, 0, len(devices))
	for _, d := range devices {
		msgs = append(msgs, expo.PushMessage{
			To:       d.PushToken,
			Title:    payload.Title,
			Body:     payload.Body,
			Sound:    payload.Sound,
			Priority: payload.Priority,
			Data:     data,
		})
	}

	tickets, err := s.expo.Send(ctx, msgs)
	if err != nil {
		logx.WithContext(ctx).Errorf("push delivery: expo send failed for user %s (%d devices): %v", userID, len(devices), err)
		return 0, fmt.Errorf("expo send: %w", err)
	}

	// Tally results, persist tickets for receipt processing, and disable stale
	// tokens immediately based on the ticket response (Expo sometimes returns
	// DeviceNotRegistered in the ticket itself, not just in the receipt).
	delivered := 0
	for i, ticket := range tickets {
		if ticket.Status == "ok" {
			delivered++
			// Persist the ticket for async receipt processing.
			if s.tickets != nil {
				if terr := s.tickets.CreatePushTicket(ctx, ticket.ID, devices[i].PushToken, userID, payload.NotificationID); terr != nil {
					logx.WithContext(ctx).Errorf("push delivery: persist ticket %s failed: %v", ticket.ID, terr)
				}
			}
			continue
		}
		logx.WithContext(ctx).Errorf("push delivery: expo ticket error for user %s device %s: %s details=%v",
			userID, devices[i].PushToken, ticket.Message, ticket.Details)
		if ticket.Details["error"] == "DeviceNotRegistered" {
			if derr := s.devices.DisableDeviceByToken(ctx, devices[i].PushToken); derr != nil {
				logx.WithContext(ctx).Errorf("push delivery: disable stale token %s failed: %v", devices[i].PushToken, derr)
			}
		}
	}

	logx.WithContext(ctx).Infof("push delivery: user %s sent=%d delivered=%d", userID, len(devices), delivered)
	return delivered, nil
}

// SendToDevices is a lower-level helper that sends a payload to an explicit
// list of devices, bypassing the user lookup. Used by broadcast/admin flows.
func (s *Sender) SendToDevices(ctx context.Context, devices []db.NotificationDevice, payload Payload) (int, error) {
	if !s.enabled || s.expo == nil || len(devices) == 0 {
		return 0, nil
	}
	data := payload.dataMap()
	msgs := make([]expo.PushMessage, 0, len(devices))
	for _, d := range devices {
		msgs = append(msgs, expo.PushMessage{
			To:       d.PushToken,
			Title:    payload.Title,
			Body:     payload.Body,
			Sound:    payload.Sound,
			Priority: payload.Priority,
			Data:     data,
		})
	}
	tickets, err := s.expo.Send(ctx, msgs)
	if err != nil {
		return 0, fmt.Errorf("expo send: %w", err)
	}
	delivered := 0
	for i, ticket := range tickets {
		if ticket.Status == "ok" {
			delivered++
			if s.tickets != nil {
				_ = s.tickets.CreatePushTicket(ctx, ticket.ID, devices[i].PushToken, devices[i].UserID, payload.NotificationID)
			}
			continue
		}
		if ticket.Details["error"] == "DeviceNotRegistered" {
			_ = s.devices.DisableDeviceByToken(ctx, devices[i].PushToken)
		}
	}
	return delivered, nil
}
