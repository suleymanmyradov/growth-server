package userdeletion

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/events"
)

type recorder struct {
	users []uuid.UUID
	err   error
}

func (r *recorder) delete(_ context.Context, userID uuid.UUID) error {
	if r.err != nil {
		return r.err
	}
	r.users = append(r.users, userID)
	return nil
}

func envelope(t *testing.T, env events.Envelope) string {
	t.Helper()
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return string(raw)
}

func validEnvelope(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	env, err := events.NewEnvelopeWithID(uuid.NewString(), events.TypeUserDeleted, events.UserDeleted{
		UserID: userID.String(),
	})
	require.NoError(t, err)
	return envelope(t, env)
}

func TestConsume_UnrelatedEventIgnored(t *testing.T) {
	rec := &recorder{}
	env, err := events.NewEnvelopeWithID(uuid.NewString(), events.TypeUserProfileUpdated, events.UserProfileUpdated{
		UserID:   uuid.NewString(),
		Username: "u",
		Email:    "e",
	})
	require.NoError(t, err)
	require.NoError(t, Handler{Delete: rec.delete}.Consume(context.Background(), "", envelope(t, env)))
	assert.Empty(t, rec.users)
}

func TestConsume_MalformedIgnored(t *testing.T) {
	rec := &recorder{}
	h := Handler{Delete: rec.delete}
	for _, raw := range []string{"not json", `{"eventType":"user_deleted"}`, `[]`} {
		require.NoError(t, h.Consume(context.Background(), "", raw))
	}
	assert.Empty(t, rec.users)
}

func TestConsume_UnsupportedVersionIgnored(t *testing.T) {
	rec := &recorder{}
	userID := uuid.New()
	env, err := events.NewEnvelopeWithID(uuid.NewString(), events.TypeUserDeleted, events.UserDeleted{
		UserID: userID.String(),
	})
	require.NoError(t, err)
	env.Version = 2
	require.NoError(t, Handler{Delete: rec.delete}.Consume(context.Background(), "", envelope(t, env)))
	assert.Empty(t, rec.users)
}

func TestConsume_InvalidIDsIgnored(t *testing.T) {
	rec := &recorder{}
	h := Handler{Delete: rec.delete}

	env, err := events.NewEnvelopeWithID(uuid.NewString(), events.TypeUserDeleted, events.UserDeleted{
		UserID: uuid.NewString(),
	})
	require.NoError(t, err)

	badEventID := env
	badEventID.EventID = uuid.Nil.String()
	require.NoError(t, h.Consume(context.Background(), "", envelope(t, badEventID)))

	badUserID := env
	badUserID.Payload = json.RawMessage(`{"userId":"00000000-0000-0000-0000-000000000000"}`)
	require.NoError(t, h.Consume(context.Background(), "", envelope(t, badUserID)))

	assert.Empty(t, rec.users)
}

func TestConsume_ValidDeletesExactUser(t *testing.T) {
	rec := &recorder{}
	userID := uuid.New()
	require.NoError(t, Handler{Delete: rec.delete}.Consume(context.Background(), "", validEnvelope(t, userID)))
	require.Equal(t, []uuid.UUID{userID}, rec.users)
}

func TestConsume_DeleteErrorPropagates(t *testing.T) {
	rec := &recorder{err: errors.New("delete failed")}
	err := Handler{Delete: rec.delete}.Consume(context.Background(), "", validEnvelope(t, uuid.New()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
}

func TestConsume_DuplicateDeletesAgain(t *testing.T) {
	rec := &recorder{}
	userID := uuid.New()
	raw := validEnvelope(t, userID)
	h := Handler{Delete: rec.delete}
	require.NoError(t, h.Consume(context.Background(), "", raw))
	require.NoError(t, h.Consume(context.Background(), "", raw))
	assert.Equal(t, []uuid.UUID{userID, userID}, rec.users)
}

func TestNewQueue_Disabled(t *testing.T) {
	q, closeFn, err := NewQueue(Config{}, Handler{})
	require.NoError(t, err)
	assert.Nil(t, q)
	assert.NotNil(t, closeFn)
	closeFn()
}

func TestNewQueue_GroupRequired(t *testing.T) {
	_, _, err := NewQueue(Config{Topic: "growth.events"}, Handler{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Group")
}

func TestNewQueue_NoTransport(t *testing.T) {
	_, _, err := NewQueue(Config{Topic: "growth.events", Group: "g"}, Handler{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no transport")
}
