package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidEventType(t *testing.T) {
	assert.True(t, ValidEventType(TypeCheckInCreated))
	assert.True(t, ValidEventType(TypeUserOnboarded))
	assert.False(t, ValidEventType("unknown_event"))
}

func TestEnvelopeValidate_OK(t *testing.T) {
	env, err := NewEnvelope(TypeCheckInCreated, CheckInCreated{UserID: "u1", HabitID: "h1"})
	require.NoError(t, err)
	assert.NoError(t, env.Validate())
}

func TestEnvelopeValidate_UnknownType(t *testing.T) {
	env := Envelope{EventType: "unknown_type", Version: 1, Payload: []byte(`{}`)}
	assert.ErrorContains(t, env.Validate(), "unknown event type")
}

func TestEnvelopeValidate_UnsupportedVersion(t *testing.T) {
	env, err := NewEnvelope(TypeCheckInCreated, CheckInCreated{UserID: "u1"})
	require.NoError(t, err)
	env.Version = 99
	assert.ErrorContains(t, env.Validate(), "unsupported version")
}

func TestEnvelopeValidate_EmptyPayload(t *testing.T) {
	env := Envelope{EventType: string(TypeCheckInCreated), Version: 1}
	assert.ErrorContains(t, env.Validate(), "payload is empty")
}

func TestEnvelopeValidate_InvalidPayload(t *testing.T) {
	env := Envelope{EventType: string(TypeCheckInCreated), Version: 1, Payload: []byte(`{`)}
	assert.ErrorContains(t, env.Validate(), "payload is not valid JSON")
}
