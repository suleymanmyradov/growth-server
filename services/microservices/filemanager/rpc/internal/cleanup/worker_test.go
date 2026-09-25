package cleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/repository/db"
)

type fakeStore struct {
	rows       []db.FileObject
	listErr    error
	byOwner    []db.FileObject
	byOwnerErr error
	deleted    []string
	deleteErr  error
	deleteCall int
}

func (f *fakeStore) ListExpiredFileObjects(context.Context, int32) ([]db.FileObject, error) {
	return f.rows, f.listErr
}

func (f *fakeStore) DeleteFileObject(_ context.Context, bucket, key string) error {
	f.deleteCall++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, bucket+"/"+key)
	return nil
}

// Unused Querier methods — fakeStore only needs the sweeper's surface.
func (f *fakeStore) CreateFileObject(context.Context, db.CreateFileObjectParams) error {
	return nil
}
func (f *fakeStore) GetFileObject(context.Context, string, string) (db.FileObject, error) {
	return db.FileObject{}, pgx.ErrNoRows
}
func (f *fakeStore) ListFileObjectsByOwner(context.Context, uuid.NullUUID) ([]db.FileObject, error) {
	return f.byOwner, f.byOwnerErr
}

type fakeRemover struct {
	removed []string
	err     error
}

func (f *fakeRemover) RemoveObject(_ context.Context, bucket, key string, _ minio.RemoveObjectOptions) error {
	if f.err != nil {
		return f.err
	}
	f.removed = append(f.removed, bucket+"/"+key)
	return nil
}

func expiredRow(bucket, key string) db.FileObject {
	return db.FileObject{
		Bucket:    bucket,
		ObjectKey: key,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	}
}

func TestSweepOnce_RemovesExpiredObjects(t *testing.T) {
	store := &fakeStore{rows: []db.FileObject{
		expiredRow("b", "exports/a.txt"),
		expiredRow("b", "exports/b.txt"),
	}}
	remover := &fakeRemover{}

	w := NewWorker(store, remover, time.Minute, 100)
	n, err := w.SweepOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []string{"b/exports/a.txt", "b/exports/b.txt"}, remover.removed)
	assert.Equal(t, remover.removed, store.deleted) // every removed object lost its row
}

func TestSweepOnce_RemoveErrorStopsBeforeRowDelete(t *testing.T) {
	store := &fakeStore{rows: []db.FileObject{expiredRow("b", "exports/a.txt")}}
	remover := &fakeRemover{err: errors.New("minio down")}

	w := NewWorker(store, remover, time.Minute, 100)
	n, err := w.SweepOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, store.deleteCall, "row must survive so the sweep retries")
}

func TestSweepOnce_RowDeleteError(t *testing.T) {
	store := &fakeStore{
		rows:      []db.FileObject{expiredRow("b", "exports/a.txt")},
		deleteErr: errors.New("db down"),
	}
	remover := &fakeRemover{}

	w := NewWorker(store, remover, time.Minute, 100)
	n, err := w.SweepOnce(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Len(t, remover.removed, 1)
}

func TestSweepOnce_NothingExpired(t *testing.T) {
	w := NewWorker(&fakeStore{}, &fakeRemover{}, time.Minute, 100)
	n, err := w.SweepOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestNewWorker_Defaults(t *testing.T) {
	w := NewWorker(&fakeStore{}, &fakeRemover{}, 0, 0)
	assert.Equal(t, defaultInterval, w.interval)
	assert.Equal(t, int32(defaultBatchSize), w.batchSize)
}

func TestDeleteUserObjects_RemovesAllOwned(t *testing.T) {
	userID := uuid.New()
	store := &fakeStore{byOwner: []db.FileObject{
		expiredRow("b", "avatars/pic.png"),
		expiredRow("b", "exports/export.json"),
	}}
	remover := &fakeRemover{}

	err := DeleteUserObjects(context.Background(), store, remover, userID)
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"b/avatars/pic.png", "b/exports/export.json"},
		remover.removed)
	assert.Equal(t, remover.removed, store.deleted)
}

func TestDeleteUserObjects_ListsByThatUser(t *testing.T) {
	userID := uuid.New()
	var gotOwner uuid.NullUUID
	store := &fakeStore{}
	// Wrap to capture the owner filter.
	capturing := &captureOwnerStore{Store: store, seen: &gotOwner}

	err := DeleteUserObjects(context.Background(), capturing, &fakeRemover{}, userID)
	require.NoError(t, err)
	require.True(t, gotOwner.Valid)
	assert.Equal(t, userID, gotOwner.UUID)
}

type captureOwnerStore struct {
	Store
	seen *uuid.NullUUID
}

func (c *captureOwnerStore) ListFileObjectsByOwner(ctx context.Context, owner uuid.NullUUID) ([]db.FileObject, error) {
	*c.seen = owner
	return c.Store.ListFileObjectsByOwner(ctx, owner)
}

func TestDeleteUserObjects_RemoveErrorPropagates(t *testing.T) {
	store := &fakeStore{byOwner: []db.FileObject{expiredRow("b", "avatars/pic.png")}}
	remover := &fakeRemover{err: errors.New("minio down")}

	err := DeleteUserObjects(context.Background(), store, remover, uuid.New())
	require.Error(t, err)
	assert.Equal(t, 0, store.deleteCall)
}

func TestDeleteUserObjects_NilStore(t *testing.T) {
	err := DeleteUserObjects(context.Background(), nil, &fakeRemover{}, uuid.New())
	require.Error(t, err)
}

func TestDeleteUserObjects_NoObjects(t *testing.T) {
	err := DeleteUserObjects(context.Background(), &fakeStore{}, &fakeRemover{}, uuid.New())
	require.NoError(t, err)
}
