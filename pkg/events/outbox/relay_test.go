package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/suleymanmyradov/growth-server/pkg/events"
)

type fakeStore struct {
	rows        []Row
	claimed     []Row
	completed   []uuid.UUID
	claimErr    error
	completeErr error
}

func (f *fakeStore) Claim(_ context.Context) (Row, error) {
	if f.claimErr != nil {
		return Row{}, f.claimErr
	}
	if len(f.rows) == 0 {
		return Row{}, pgx.ErrNoRows
	}
	row := f.rows[0]
	f.claimed = append(f.claimed, row)
	return row, nil
}

func (f *fakeStore) Complete(_ context.Context, eventID uuid.UUID) error {
	f.completed = append(f.completed, eventID)
	return f.completeErr
}

type fakePublisher struct {
	envs []events.Envelope
	err  error
}

func (f *fakePublisher) Publish(_ context.Context, env events.Envelope) error {
	if f.err != nil {
		return f.err
	}
	f.envs = append(f.envs, env)
	return nil
}

func newRow(t *testing.T, eventType string, payload []byte) Row {
	t.Helper()
	return Row{
		EventID:    uuid.New(),
		EventType:  eventType,
		Payload:    payload,
		OccurredAt: time.Now(),
	}
}

func TestRunOnce_PublishesAndCompletes(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"user_id": "u1"})
	row := newRow(t, string(events.TypeCheckInCreated), payload)
	store := &fakeStore{rows: []Row{row}}
	pub := &fakePublisher{}

	claimed, err := NewRelay("test", store, pub).RunOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, claimed)

	require.Len(t, pub.envs, 1)
	env := pub.envs[0]
	assert.Equal(t, row.EventID.String(), env.EventID, "relay must republish under the stored event ID")
	assert.Equal(t, row.EventType, env.EventType)
	assert.JSONEq(t, string(payload), string(env.Payload))
	assert.Equal(t, []uuid.UUID{row.EventID}, store.completed, "row deleted only after successful publish")
}

func TestRunOnce_EmptyOutboxReturnsUnclaimed(t *testing.T) {
	store := &fakeStore{}
	claimed, err := NewRelay("test", store, &fakePublisher{}).RunOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, claimed)
}

func TestRunOnce_PublishFailureKeepsRow(t *testing.T) {
	row := newRow(t, string(events.TypeCheckInCreated), []byte(`{}`))
	store := &fakeStore{rows: []Row{row}}
	pub := &fakePublisher{err: errors.New("broker down")}

	claimed, err := NewRelay("test", store, pub).RunOnce(context.Background())
	require.Error(t, err)
	assert.False(t, claimed)
	assert.Empty(t, store.completed, "row must remain claimable for retry")
}

func TestRunOnce_CompleteFailureReportsClaimed(t *testing.T) {
	// Publish succeeded but the delete failed: the row stays claimable, the
	// event gets republished, and consumers dedupe by event ID.
	row := newRow(t, string(events.TypeCheckInCreated), []byte(`{}`))
	store := &fakeStore{rows: []Row{row}, completeErr: errors.New("db hiccup")}
	pub := &fakePublisher{}

	claimed, err := NewRelay("test", store, pub).RunOnce(context.Background())
	require.Error(t, err)
	assert.True(t, claimed, "the event WAS delivered — callers must not treat this as undelivered")
	assert.Len(t, pub.envs, 1)
}

func TestRunOnce_ClaimErrorPropagates(t *testing.T) {
	store := &fakeStore{claimErr: errors.New("db unreachable")}
	claimed, err := NewRelay("test", store, &fakePublisher{}).RunOnce(context.Background())
	require.Error(t, err)
	assert.False(t, claimed)
}

func TestRunOnce_NilPublisher(t *testing.T) {
	_, err := NewRelay("test", &fakeStore{}, nil).RunOnce(context.Background())
	require.Error(t, err)
}

func TestEnqueueParams_RoundTrip(t *testing.T) {
	env := events.Envelope{
		EventID:    uuid.NewString(),
		EventType:  string(events.TypeHabitCreated),
		Version:    1,
		OccurredAt: time.Now().UTC().Truncate(time.Microsecond),
		Payload:    json.RawMessage(`{"a":1}`),
	}
	eventID, eventType, payload, occurredAt, err := EnqueueParams(env)
	require.NoError(t, err)
	assert.Equal(t, env.EventID, eventID.String())
	assert.Equal(t, env.EventType, eventType)
	assert.Equal(t, []byte(env.Payload), payload)
	assert.True(t, occurredAt.Valid)
	assert.Equal(t, env.OccurredAt, occurredAt.Time)
}

func TestEnqueueParams_InvalidEventID(t *testing.T) {
	_, _, _, _, err := EnqueueParams(events.Envelope{EventID: "not-a-uuid"})
	require.Error(t, err)
}
