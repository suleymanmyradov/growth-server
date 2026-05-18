package events

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnvelope(t *testing.T) {
	payload := CheckInCreated{
		UserID:    "user-1",
		HabitID:   "habit-1",
		HabitName: "Meditation",
		Status:    "done",
		Streak:    7,
	}

	env, err := NewEnvelope(TypeCheckInCreated, payload)
	require.NoError(t, err)

	assert.NotEmpty(t, env.EventID, "eventId should be populated")
	assert.Equal(t, string(TypeCheckInCreated), env.EventType)
	assert.Equal(t, 1, env.Version)
	assert.NotZero(t, env.OccurredAt, "occurredAt should be set")
	assert.NotEmpty(t, env.Payload, "payload should be marshalled")

	var got CheckInCreated
	require.NoError(t, json.Unmarshal(env.Payload, &got), "payload should unmarshal")
	assert.Equal(t, payload, got)
}

func TestEnvelopeRoundTrip(t *testing.T) {
	payload := SettingsChanged{
		UserID:         "user-2",
		Timezone:       "America/New_York",
		CheckInTime:    "09:00:00",
		HabitReminders: true,
	}

	env, err := NewEnvelope(TypeSettingsChanged, payload)
	require.NoError(t, err)

	data, err := json.Marshal(env)
	require.NoError(t, err, "envelope should marshal to JSON")

	var decoded Envelope
	require.NoError(t, json.Unmarshal(data, &decoded), "envelope should unmarshal from JSON")
	assert.Equal(t, env.EventID, decoded.EventID)
	assert.Equal(t, env.EventType, decoded.EventType)
	assert.Equal(t, env.Version, decoded.Version)
	assert.WithinDuration(t, env.OccurredAt, decoded.OccurredAt, 0)

	var got SettingsChanged
	require.NoError(t, json.Unmarshal(decoded.Payload, &got))
	assert.Equal(t, payload, got)
}

func TestCheckInFeedbackGeneratedRoundTrip(t *testing.T) {
	payload := CheckInFeedbackGenerated{
		UserID:    "user-3",
		CheckInID: "checkin-1",
		HabitID:   "habit-1",
		Content:   "Great work on your meditation streak!",
	}

	env, err := NewEnvelope(TypeCheckInFeedbackGenerated, payload)
	require.NoError(t, err)

	data, err := json.Marshal(env)
	require.NoError(t, err, "envelope should marshal to JSON")

	var decoded Envelope
	require.NoError(t, json.Unmarshal(data, &decoded), "envelope should unmarshal from JSON")
	assert.Equal(t, string(TypeCheckInFeedbackGenerated), decoded.EventType)

	var got CheckInFeedbackGenerated
	require.NoError(t, json.Unmarshal(decoded.Payload, &got))
	assert.Equal(t, payload, got)
}

func TestNewEnvelopeUUIDGeneration(t *testing.T) {
	env1, err := NewEnvelope(TypeReminderDue, ReminderDue{UserID: "u1"})
	require.NoError(t, err)

	env2, err := NewEnvelope(TypeReminderDue, ReminderDue{UserID: "u2"})
	require.NoError(t, err)

	assert.NotEqual(t, env1.EventID, env2.EventID, "each envelope should have a unique event ID")
}
