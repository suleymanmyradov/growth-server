// Package cleanup deletes objects whose registry row is past its retention
// deadline (file_objects.expires_at). The sweeper is the only thing that
// enforces retention for non-public prefixes like exports/ — presigned URLs
// expire in minutes, but the object itself would otherwise sit in the bucket
// forever.
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultInterval  = 5 * time.Minute
	defaultBatchSize = 100
)

// Store is the registry persistence the sweeper and the user-deletion fan-out
// need (satisfied by *db.Queries).
type Store interface {
	ListExpiredFileObjects(context.Context, int32) ([]db.FileObject, error)
	ListFileObjectsByOwner(context.Context, uuid.NullUUID) ([]db.FileObject, error)
	DeleteFileObject(context.Context, string, string) error
}

// Remover deletes an object from the bucket (satisfied by *minio.Client).
// Removal is idempotent: deleting a missing key succeeds.
type Remover interface {
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

// Worker sweeps expired objects. Order per row is object-then-row: a row that
// survives a failed RemoveObject is retried next pass, while a missing object
// still lets the row be reclaimed (RemoveObject succeeds on absent keys).
type Worker struct {
	store     Store
	objects   Remover
	interval  time.Duration
	batchSize int32
	cancel    context.CancelFunc
}

func NewWorker(store Store, objects Remover, interval time.Duration, batchSize int) *Worker {
	if interval <= 0 {
		interval = defaultInterval
	}
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	return &Worker{
		store:     store,
		objects:   objects,
		interval:  interval,
		batchSize: int32(batchSize),
	}
}

// SweepOnce deletes one batch of expired objects and returns how many rows
// were reclaimed.
func (w *Worker) SweepOnce(ctx context.Context) (int, error) {
	if w.store == nil || w.objects == nil {
		return 0, errors.New("cleanup worker: nil store or remover")
	}
	rows, err := w.store.ListExpiredFileObjects(ctx, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("list expired objects: %w", err)
	}
	reclaimed := 0
	for _, row := range rows {
		if err := w.objects.RemoveObject(ctx, row.Bucket, row.ObjectKey, minio.RemoveObjectOptions{}); err != nil {
			return reclaimed, fmt.Errorf("remove object %s/%s: %w", row.Bucket, row.ObjectKey, err)
		}
		if err := w.store.DeleteFileObject(ctx, row.Bucket, row.ObjectKey); err != nil {
			return reclaimed, fmt.Errorf("delete registry row %s/%s: %w", row.Bucket, row.ObjectKey, err)
		}
		reclaimed++
	}
	return reclaimed, nil
}

// DeleteUserObjects removes every object owned by userID — the filemanager
// side of a user_deleted event. Each step is idempotent: the object delete
// succeeds on absent keys and the row delete is a no-op on absent rows, so
// redelivery after a partial pass just resumes. A nil store means Postgres is
// unconfigured and ownership was never tracked — nothing can be found, which
// is an error worth redelivering rather than silently succeeding.
func DeleteUserObjects(ctx context.Context, store Store, objects Remover, userID uuid.UUID) error {
	if store == nil || objects == nil {
		return errors.New("user deletion cleanup unavailable: postgres not configured")
	}
	rows, err := store.ListFileObjectsByOwner(ctx, uuid.NullUUID{UUID: userID, Valid: true})
	if err != nil {
		return fmt.Errorf("list objects for user %s: %w", userID, err)
	}
	for _, row := range rows {
		if err := objects.RemoveObject(ctx, row.Bucket, row.ObjectKey, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("remove object %s/%s: %w", row.Bucket, row.ObjectKey, err)
		}
		if err := store.DeleteFileObject(ctx, row.Bucket, row.ObjectKey); err != nil {
			return fmt.Errorf("delete registry row %s/%s: %w", row.Bucket, row.ObjectKey, err)
		}
	}
	if len(rows) > 0 {
		logx.WithContext(ctx).Infof("user_deleted: removed %d object(s) for user %s", len(rows), userID)
	}
	return nil
}

// Start launches the sweep loop in the background. No-op if already running.
func (w *Worker) Start() {
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	go w.run(ctx)
}

// Stop cancels the sweep loop. Safe to call when not started.
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
}

// run sweeps immediately, then on every interval. A failed pass is logged and
// the loop waits for the next tick rather than busy-looping.
func (w *Worker) run(ctx context.Context) {
	for {
		n, err := w.SweepOnce(ctx)
		if err != nil {
			logx.WithContext(ctx).Errorf("cleanup sweep: %v", err)
		} else if n > 0 {
			logx.WithContext(ctx).Infof("cleanup sweep reclaimed %d expired object(s)", n)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(w.interval):
		}
	}
}
