package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/analytics-consumer/internal/repository/db"
)

type Repository struct {
	queries *db.Queries
}

func NewRepository(q *db.Queries) *Repository {
	return &Repository{queries: q}
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return NewRepository(db.NewWithTx(tx))
}

func (r *Repository) IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	return r.queries.IsProcessed(ctx, eventID)
}

func (r *Repository) MarkProcessed(ctx context.Context, eventID uuid.UUID) error {
	return r.queries.MarkProcessed(ctx, eventID)
}

func (r *Repository) InsertLifecycleEvent(ctx context.Context, e db.LifecycleEvent) error {
	return r.queries.InsertLifecycleEvent(ctx, e)
}

func (r *Repository) UpsertDailyMetric(ctx context.Context, date time.Time, name string, value int64, metadata []byte) error {
	return r.queries.UpsertDailyMetric(ctx, date, name, value, metadata)
}

func (r *Repository) UpsertConversionFunnelStage(ctx context.Context, s db.ConversionFunnelStage) error {
	return r.queries.UpsertConversionFunnelStage(ctx, s)
}

func (r *Repository) UpsertRetentionCohort(ctx context.Context, c db.RetentionCohort) error {
	return r.queries.UpsertRetentionCohort(ctx, c)
}

func (r *Repository) GetDailyMetrics(ctx context.Context, from, to time.Time) ([]db.MetricRow, error) {
	return r.queries.GetDailyMetrics(ctx, from, to)
}

func (r *Repository) GetLifecycleCounts(ctx context.Context, from, to time.Time) ([]db.LifecycleCount, error) {
	return r.queries.GetLifecycleCounts(ctx, from, to)
}

func (r *Repository) GetConversionFunnelCounts(ctx context.Context, from, to time.Time) ([]db.ConversionStageCount, error) {
	return r.queries.GetConversionFunnelCounts(ctx, from, to)
}

func (r *Repository) GetRetentionCohorts(ctx context.Context, from, to time.Time) ([]db.RetentionCohortRow, error) {
	return r.queries.GetRetentionCohorts(ctx, from, to)
}

func (r *Repository) DeleteLifecycleEventsByUser(ctx context.Context, userID uuid.UUID) error {
	return r.queries.DeleteLifecycleEventsByUser(ctx, userID)
}

func (r *Repository) DeleteConversionFunnelByUser(ctx context.Context, userID uuid.UUID) error {
	return r.queries.DeleteConversionFunnelByUser(ctx, userID)
}
