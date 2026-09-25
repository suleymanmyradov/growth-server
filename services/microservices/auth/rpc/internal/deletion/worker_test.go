package deletion

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

type fakeStore struct {
	mu        sync.Mutex
	claimErr  error
	job       db.ClaimAuthDeletionRow
	claims    int
	completed []uuid.UUID
	compErr   error
}

func (f *fakeStore) ClaimAuthDeletion(context.Context) (db.ClaimAuthDeletionRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims++
	return f.job, f.claimErr
}

func (f *fakeStore) CompleteAuthDeletion(_ context.Context, eventID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completed = append(f.completed, eventID)
	return f.compErr
}

func (f *fakeStore) claimCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.claims
}

func (f *fakeStore) completedIDs() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]uuid.UUID(nil), f.completed...)
}

type fakePublisher struct {
	mu        sync.Mutex
	published []events.Envelope
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, env events.Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err == nil {
		f.published = append(f.published, env)
	}
	return f.err
}

func (f *fakePublisher) envelopes() []events.Envelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]events.Envelope(nil), f.published...)
}

func newJob(t *testing.T) db.ClaimAuthDeletionRow {
	t.Helper()
	eventID, err := uuid.NewV7()
	require.NoError(t, err)
	userID, err := uuid.NewV7()
	require.NoError(t, err)
	return db.ClaimAuthDeletionRow{
		EventID:   eventID,
		UserID:    userID,
		CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), Valid: true},
	}
}

func TestRunOnce_NoRow(t *testing.T) {
	store := &fakeStore{claimErr: pgx.ErrNoRows}
	pub := &fakePublisher{}
	claimed, err := NewWorker(store, pub).RunOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Equal(t, 1, store.claimCount())
	assert.Empty(t, pub.envelopes())
	assert.Empty(t, store.completedIDs())
}

func TestRunOnce_ClaimFailure(t *testing.T) {
	store := &fakeStore{claimErr: errors.New("db down")}
	pub := &fakePublisher{}
	claimed, err := NewWorker(store, pub).RunOnce(context.Background())
	require.Error(t, err)
	assert.False(t, claimed)
	assert.Empty(t, pub.envelopes())
	assert.Empty(t, store.completedIDs())
}

func TestRunOnce_SuccessPublishesAndCompletes(t *testing.T) {
	job := newJob(t)
	store := &fakeStore{job: job}
	pub := &fakePublisher{}

	claimed, err := NewWorker(store, pub).RunOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, claimed)

	envs := pub.envelopes()
	require.Len(t, envs, 1)
	env := envs[0]
	assert.Equal(t, job.EventID.String(), env.EventID)
	assert.Equal(t, string(events.TypeUserDeleted), env.EventType)
	assert.Equal(t, 1, env.Version)
	assert.Equal(t, job.CreatedAt.Time, env.OccurredAt)
	assert.JSONEq(t, `{"userId":"`+job.UserID.String()+`"}`, string(env.Payload))

	completed := store.completedIDs()
	require.Len(t, completed, 1)
	assert.Equal(t, job.EventID, completed[0])
}

func TestRunOnce_PublishFailureNoAck(t *testing.T) {
	store := &fakeStore{job: newJob(t)}
	pub := &fakePublisher{err: errors.New("broker down")}
	claimed, err := NewWorker(store, pub).RunOnce(context.Background())
	require.Error(t, err)
	assert.False(t, claimed)
	assert.Empty(t, store.completedIDs())
}

func TestRunOnce_CompleteFailureReturnsError(t *testing.T) {
	job := newJob(t)
	store := &fakeStore{job: job, compErr: errors.New("ack failed")}
	pub := &fakePublisher{}
	claimed, err := NewWorker(store, pub).RunOnce(context.Background())
	require.Error(t, err)
	assert.True(t, claimed)
	require.Len(t, pub.envelopes(), 1)
}

func TestRunOnce_RetryReusesSameEventID(t *testing.T) {
	job := newJob(t)
	store := &fakeStore{job: job}
	pub := &fakePublisher{}
	w := NewWorker(store, pub)

	for i := 0; i < 3; i++ {
		claimed, err := w.RunOnce(context.Background())
		require.NoError(t, err)
		assert.True(t, claimed)
	}
	for _, env := range pub.envelopes() {
		assert.Equal(t, job.EventID.String(), env.EventID)
	}
}

func TestRunOnce_NilPublisherDoesNotClaim(t *testing.T) {
	store := &fakeStore{job: newJob(t)}
	claimed, err := NewWorker(store, nil).RunOnce(context.Background())
	require.Error(t, err)
	assert.False(t, claimed)
	assert.Equal(t, 0, store.claimCount())
}

func TestRun_CancelledExitsBounded(t *testing.T) {
	store := &fakeStore{claimErr: pgx.ErrNoRows}
	pub := &fakePublisher{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		NewWorker(store, pub).Run(ctx)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}
