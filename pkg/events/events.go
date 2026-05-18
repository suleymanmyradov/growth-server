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
	TypeCheckInCreated  EventType = "check_in_created"
	TypeUserOnboarded   EventType = "user_onboarded"
	TypeSettingsChanged EventType = "settings_changed"
	TypeReminderDue              EventType = "reminder_due"
	TypeCheckInFeedbackGenerated EventType = "check_in_feedback_generated"
)

// Envelope wraps every event published to Kafka with stable metadata.
// Consumers must inspect EventType to determine how to decode Payload.
type Envelope struct {
	EventID     string          `json:"eventId"`
	EventType   string          `json:"eventType"`
	Version     int             `json:"version"`
	OccurredAt  time.Time       `json:"occurredAt"`
	Payload     json.RawMessage `json:"payload"`
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
	UserID        string `json:"userId"`
	Timezone      string `json:"timezone"`
	CheckInTime   string `json:"checkInTime"`
	HabitReminders bool  `json:"habitReminders"`
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
	ReminderID   string `json:"reminderId"`
	UserID       string `json:"userId"`
	Type         string `json:"type"`
	ScheduledAt  string `json:"scheduledAt"`
	Metadata     string `json:"metadata,omitempty"`
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
