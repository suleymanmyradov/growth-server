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
	TypeHabitCreated             EventType = "habit_created"
	TypeHabitDeleted             EventType = "habit_deleted"
	TypeUserDeleted              EventType = "user_deleted"
	TypeUserProfileUpdated       EventType = "user_profile_updated"
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
	UserID         string `json:"userId"`
	Timezone       string `json:"timezone"`
	CheckInTime    string `json:"checkInTime"`
	HabitReminders bool   `json:"habitReminders"`
}

// CheckInFeedbackGenerated is the payload for TypeCheckInFeedbackGenerated events.
type CheckInFeedbackGenerated struct {
	UserID    string `json:"userId"`
	CheckInID string `json:"checkInId"`
	HabitID   string `json:"habitId"`
	Content   string `json:"content"`
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
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}
	return Envelope{
		EventID:    id.String(),
		EventType:  string(eventType),
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Payload:    raw,
	}, nil
}
