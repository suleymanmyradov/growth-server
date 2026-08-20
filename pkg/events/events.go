package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Topic constants for Kafka topics used across services.
const (
	TopicEvents      = "growth.events"
	TopicReminderDue = "growth.reminder.due"
)

// EventType identifies the kind of domain event carried in an Envelope.
type EventType string

const (
	TypeCheckInCreated           EventType = "check_in_created"
	TypeUserOnboarded            EventType = "user_onboarded"
	TypeSettingsChanged          EventType = "settings_changed"
	TypeReminderDue              EventType = "reminder_due"
	TypeCheckInFeedbackGenerated EventType = "check_in_feedback_generated"
	// TypeCoachDigestRequested is published by the notifications service when a
	// coach_digest reminder fires (scheduled at check_in_time + 2h). The
	// ai-coach-consumer fetches all of the user's check-ins for that date and
	// generates a single combined feedback message, replacing the old
	// per-check-in feedback that spammed users with N notifications for N habits.
	TypeCoachDigestRequested EventType = "coach_digest_requested"
	TypeHabitCreated         EventType = "habit_created"
	TypeHabitDeleted         EventType = "habit_deleted"
	TypeUserDeleted          EventType = "user_deleted"
	TypeUserProfileUpdated   EventType = "user_profile_updated"
	// Goal lifecycle events — published by the client service.
	TypeGoalCreated   EventType = "goal_created"
	TypeGoalCompleted EventType = "goal_completed"
	TypeGoalDeleted   EventType = "goal_deleted"
	// TypeSubscriptionChanged is published by the client billing service when
	// a subscription status changes (upgrade, downgrade, churn, trial start).
	TypeSubscriptionChanged EventType = "subscription_changed"
	// TypePlanAdjustmentCreated is published when a plan adjustment suggestion
	// is created (by weekly review, missed-day recovery, or manual).
	TypePlanAdjustmentCreated EventType = "plan_adjustment_created"
	// TypeBroadcastNotificationRequested is published by adminway when an admin
	// sends a notification to a (segmented) audience. Payload: BroadcastNotificationRequested.
	// The notifications consumer fan-outs the notification to each user id in the
	// payload by batch-inserting rows into the notifications table.
	TypeBroadcastNotificationRequested EventType = "broadcast_notification_requested"
)

// Envelope wraps every event published to Kafka with stable metadata.
// Consumers must inspect EventType to determine how to decode Payload.
type Envelope struct {
	EventID    string          `json:"eventId"`
	EventType  string          `json:"eventType"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload"`
}

// CheckInCreated is the payload for TypeCheckInCreated events.
type CheckInCreated struct {
	UserID    string `json:"userId"`
	CheckInID string `json:"checkInId"`
	HabitID   string `json:"habitId"`
	HabitName string `json:"habitName"`
	Status    string `json:"status"`
	Streak    int32  `json:"streak"`
}

// UserOnboarded is the payload for TypeUserOnboarded events.
type UserOnboarded struct {
	UserID string `json:"userId"`
}

// SettingsChanged is the payload for TypeSettingsChanged events.
type SettingsChanged struct {
	UserID      string `json:"userId"`
	Timezone    string `json:"timezone"`
	CheckInTime string `json:"checkInTime"`
}

// CheckInFeedbackGenerated is the payload for TypeCheckInFeedbackGenerated events.
type CheckInFeedbackGenerated struct {
	UserID    string `json:"userId"`
	CheckInID string `json:"checkInId"`
	HabitID   string `json:"habitId"`
	Content   string `json:"content"`
}

// CoachDigestRequested is the payload for TypeCoachDigestRequested events.
// Published by the notifications service when a coach_digest reminder fires.
// The ai-coach-consumer uses Date (YYYY-MM-DD in the user's timezone) to fetch
// the correct day's check-ins and generate one combined feedback message.
type CoachDigestRequested struct {
	UserID string `json:"userId"`
	Date   string `json:"date"` // YYYY-MM-DD in the user's local timezone
}

// ReminderDue is the payload for TypeReminderDue events.
type ReminderDue struct {
	ReminderID  string `json:"reminderId"`
	UserID      string `json:"userId"`
	Type        string `json:"type"`
	ScheduledAt string `json:"scheduledAt"`
	Metadata    string `json:"metadata,omitempty"`
}

// HabitCreated is the payload for TypeHabitCreated events.
type HabitCreated struct {
	UserID  string `json:"userId"`
	HabitID string `json:"habitId"`
}

// HabitDeleted is the payload for TypeHabitDeleted events.
type HabitDeleted struct {
	UserID  string `json:"userId"`
	HabitID string `json:"habitId"`
}

// GoalCreated is the payload for TypeGoalCreated events.
type GoalCreated struct {
	UserID   string `json:"userId"`
	GoalID   string `json:"goalId"`
	Title    string `json:"title"`
	Category string `json:"category,omitempty"`
}

// GoalCompleted is the payload for TypeGoalCompleted events.
type GoalCompleted struct {
	UserID string `json:"userId"`
	GoalID string `json:"goalId"`
	Title  string `json:"title"`
}

// GoalDeleted is the payload for TypeGoalDeleted events.
type GoalDeleted struct {
	UserID string `json:"userId"`
	GoalID string `json:"goalId"`
}

// SubscriptionChanged is the payload for TypeSubscriptionChanged events.
// Published by the client billing service on subscription lifecycle changes.
type SubscriptionChanged struct {
	UserID         string `json:"userId"`
	PlanCode       string `json:"planCode"`
	PreviousStatus string `json:"previousStatus,omitempty"`
	NewStatus      string `json:"newStatus"`
	BillingInterval string `json:"billingInterval,omitempty"`
}

// PlanAdjustmentCreated is the payload for TypePlanAdjustmentCreated events.
type PlanAdjustmentCreated struct {
	UserID         string `json:"userId"`
	SuggestionID   string `json:"suggestionId"`
	HabitID        string `json:"habitId,omitempty"`
	GoalID         string `json:"goalId,omitempty"`
	Source         string `json:"source"`
	AdjustmentType string `json:"adjustmentType"`
}

// UserDeleted is the payload for TypeUserDeleted events.
// Published by auth when a user account is deleted; consumed by every service
// that owns user-keyed tables to clean up local data.
type UserDeleted struct {
	UserID string `json:"userId"`
}

// UserProfileUpdated is the payload for TypeUserProfileUpdated events.
// Published by auth on registration, Google OAuth signup, and profile updates;
// consumed by services that maintain a local read model of user profiles.
// All fields are populated so the consumer can fully sync its read model.
type UserProfileUpdated struct {
	UserID    string   `json:"userId"`
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	Bio       string   `json:"bio,omitempty"`
	Location  string   `json:"location,omitempty"`
	Website   string   `json:"website,omitempty"`
	Interests []string `json:"interests,omitempty"`
	Avatar    string   `json:"avatar,omitempty"`
}

// BroadcastNotificationRequested is the payload for
// TypeBroadcastNotificationRequested events. Adminway resolves the audience
// (all / premium / free) into a concrete list of user ids, chunks it to keep
// the Kafka message small, and publishes one event per chunk. The
// notifications consumer batch-inserts a notification row for every user id.
//
// BroadcastId is a client-generated UUID shared by all chunks of the same
// broadcast so the consumer can dedupe / group them. ChunkIndex + ChunkTotal
// are 1-based for observability.
type BroadcastNotificationRequested struct {
	BroadcastID string   `json:"broadcastId"`
	ChunkIndex  int      `json:"chunkIndex"`
	ChunkTotal  int      `json:"chunkTotal"`
	Title       string   `json:"title"`
	Message     string   `json:"message"`
	Type        string   `json:"type"`
	UserIDs     []string `json:"userIds"`
}

// NewEnvelope creates a new Envelope with a UUID v7 event ID, the given
// event type, and the payload marshalled to JSON.
func NewEnvelope(eventType EventType, payload any) (Envelope, error) {
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.New()
	}
	return newEnvelopeWithID(id.String(), eventType, payload)
}

// NewEnvelopeWithID creates an envelope with a caller-supplied event ID. Use
// this when you need deterministic idempotency: the consumer's processed_events
// dedup keys on EventID, so a retry with the same ID is a no-op. The ID must
// be a valid UUID string.
func NewEnvelopeWithID(eventID string, eventType EventType, payload any) (Envelope, error) {
	return newEnvelopeWithID(eventID, eventType, payload)
}

func newEnvelopeWithID(eventID string, eventType EventType, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}
	return Envelope{
		EventID:    eventID,
		EventType:  string(eventType),
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Payload:    raw,
	}, nil
}
