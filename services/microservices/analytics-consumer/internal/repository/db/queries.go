package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the common interface for pgx database operations.
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type Queries struct {
	db DBTX
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

func NewWithTx(db DBTX) *Queries {
	return &Queries{db: db}
}

// ─── Processed events (idempotency) ──────────────────────────────────────────

const isProcessed = `SELECT EXISTS(SELECT 1 FROM analytics_processed_events WHERE event_id = $1)`

func (q *Queries) IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := q.db.QueryRow(ctx, isProcessed, eventID.String()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is processed: %w", err)
	}
	return exists, nil
}

const markProcessed = `INSERT INTO analytics_processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`

func (q *Queries) MarkProcessed(ctx context.Context, eventID uuid.UUID) error {
	_, err := q.db.Exec(ctx, markProcessed, eventID.String())
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

// ─── Lifecycle events ────────────────────────────────────────────────────────

type LifecycleEvent struct {
	UserID     uuid.UUID
	EventType  string
	OccurredAt time.Time
	Metadata   []byte
}

const insertLifecycleEvent = `
INSERT INTO user_lifecycle_events (user_id, event_type, occurred_at, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING`

func (q *Queries) InsertLifecycleEvent(ctx context.Context, e LifecycleEvent) error {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	if e.Metadata == nil {
		e.Metadata = []byte("{}")
	}
	_, err := q.db.Exec(ctx, insertLifecycleEvent, e.UserID, e.EventType, e.OccurredAt, e.Metadata)
	if err != nil {
		return fmt.Errorf("insert lifecycle event: %w", err)
	}
	return nil
}

// ─── Daily metrics ───────────────────────────────────────────────────────────

const upsertDailyMetric = `
INSERT INTO daily_metrics (metric_date, metric_name, metric_value, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (metric_date, metric_name) DO UPDATE SET
    metric_value = daily_metrics.metric_value + EXCLUDED.metric_value,
    metadata = EXCLUDED.metadata,
    updated_at = now()`

func (q *Queries) UpsertDailyMetric(ctx context.Context, date time.Time, name string, value int64, metadata []byte) error {
	if metadata == nil {
		metadata = []byte("{}")
	}
	_, err := q.db.Exec(ctx, upsertDailyMetric, date, name, value, metadata)
	if err != nil {
		return fmt.Errorf("upsert daily metric: %w", err)
	}
	return nil
}

// ─── Conversion funnels ──────────────────────────────────────────────────────

type ConversionFunnelStage struct {
	UserID    uuid.UUID
	Stage     string
	EnteredAt time.Time
	Metadata  []byte
}

const upsertConversionFunnelStage = `
INSERT INTO conversion_funnels (user_id, stage, entered_at, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, stage) DO NOTHING`

func (q *Queries) UpsertConversionFunnelStage(ctx context.Context, s ConversionFunnelStage) error {
	if s.EnteredAt.IsZero() {
		s.EnteredAt = time.Now().UTC()
	}
	if s.Metadata == nil {
		s.Metadata = []byte("{}")
	}
	_, err := q.db.Exec(ctx, upsertConversionFunnelStage, s.UserID, s.Stage, s.EnteredAt, s.Metadata)
	if err != nil {
		return fmt.Errorf("upsert conversion funnel stage: %w", err)
	}
	return nil
}

// ─── Retention cohorts ───────────────────────────────────────────────────────

type RetentionCohort struct {
	CohortDate    time.Time
	CohortSize    int64
	PeriodDays    int32
	RetainedCount int64
	RetentionRate float64
}

const upsertRetentionCohort = `
INSERT INTO retention_cohorts (cohort_date, cohort_size, period_days, retained_count, retention_rate)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (cohort_date, period_days) DO UPDATE SET
    cohort_size = EXCLUDED.cohort_size,
    retained_count = EXCLUDED.retained_count,
    retention_rate = EXCLUDED.retention_rate,
    updated_at = now()`

func (q *Queries) UpsertRetentionCohort(ctx context.Context, c RetentionCohort) error {
	_, err := q.db.Exec(ctx, upsertRetentionCohort, c.CohortDate, c.CohortSize, c.PeriodDays, c.RetainedCount, c.RetentionRate)
	if err != nil {
		return fmt.Errorf("upsert retention cohort: %w", err)
	}
	return nil
}

// ─── Query helpers for admin metrics ─────────────────────────────────────────

type MetricRow struct {
	MetricDate  time.Time
	MetricName  string
	MetricValue int64
}

const getDailyMetrics = `
SELECT metric_date, metric_name, metric_value
FROM daily_metrics
WHERE metric_date >= $1 AND metric_date <= $2
ORDER BY metric_date DESC, metric_name`

func (q *Queries) GetDailyMetrics(ctx context.Context, from, to time.Time) ([]MetricRow, error) {
	rows, err := q.db.Query(ctx, getDailyMetrics, from, to)
	if err != nil {
		return nil, fmt.Errorf("get daily metrics: %w", err)
	}
	defer rows.Close()
	var items []MetricRow
	for rows.Next() {
		var m MetricRow
		if err := rows.Scan(&m.MetricDate, &m.MetricName, &m.MetricValue); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

type LifecycleCount struct {
	EventType string
	Count     int64
}

const getLifecycleCounts = `
SELECT event_type, COUNT(DISTINCT user_id) as count
FROM user_lifecycle_events
WHERE occurred_at >= $1 AND occurred_at <= $2
GROUP BY event_type
ORDER BY event_type`

func (q *Queries) GetLifecycleCounts(ctx context.Context, from, to time.Time) ([]LifecycleCount, error) {
	rows, err := q.db.Query(ctx, getLifecycleCounts, from, to)
	if err != nil {
		return nil, fmt.Errorf("get lifecycle counts: %w", err)
	}
	defer rows.Close()
	var items []LifecycleCount
	for rows.Next() {
		var l LifecycleCount
		if err := rows.Scan(&l.EventType, &l.Count); err != nil {
			return nil, fmt.Errorf("scan lifecycle count: %w", err)
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

type ConversionStageCount struct {
	Stage string
	Count int64
}

const getConversionFunnelCounts = `
SELECT stage, COUNT(DISTINCT user_id) as count
FROM conversion_funnels
WHERE entered_at >= $1 AND entered_at <= $2
GROUP BY stage
ORDER BY stage`

func (q *Queries) GetConversionFunnelCounts(ctx context.Context, from, to time.Time) ([]ConversionStageCount, error) {
	rows, err := q.db.Query(ctx, getConversionFunnelCounts, from, to)
	if err != nil {
		return nil, fmt.Errorf("get conversion funnel counts: %w", err)
	}
	defer rows.Close()
	var items []ConversionStageCount
	for rows.Next() {
		var c ConversionStageCount
		if err := rows.Scan(&c.Stage, &c.Count); err != nil {
			return nil, fmt.Errorf("scan conversion count: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

type RetentionCohortRow struct {
	CohortDate    time.Time
	CohortSize    int64
	PeriodDays    int32
	RetainedCount int64
	RetentionRate float64
}

const getRetentionCohorts = `
SELECT cohort_date, cohort_size, period_days, retained_count, retention_rate
FROM retention_cohorts
WHERE cohort_date >= $1 AND cohort_date <= $2
ORDER BY cohort_date DESC, period_days`

func (q *Queries) GetRetentionCohorts(ctx context.Context, from, to time.Time) ([]RetentionCohortRow, error) {
	rows, err := q.db.Query(ctx, getRetentionCohorts, from, to)
	if err != nil {
		return nil, fmt.Errorf("get retention cohorts: %w", err)
	}
	defer rows.Close()
	var items []RetentionCohortRow
	for rows.Next() {
		var r RetentionCohortRow
		if err := rows.Scan(&r.CohortDate, &r.CohortSize, &r.PeriodDays, &r.RetainedCount, &r.RetentionRate); err != nil {
			return nil, fmt.Errorf("scan retention cohort: %w", err)
		}
		items = append(items, r)
	}
	return items, rows.Err()
}

// ─── User cleanup (on account deletion) ──────────────────────────────────────

const deleteLifecycleEventsByUser = `DELETE FROM user_lifecycle_events WHERE user_id = $1`

func (q *Queries) DeleteLifecycleEventsByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteLifecycleEventsByUser, userID)
	return err
}

const deleteConversionFunnelByUser = `DELETE FROM conversion_funnels WHERE user_id = $1`

func (q *Queries) DeleteConversionFunnelByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteConversionFunnelByUser, userID)
	return err
}
