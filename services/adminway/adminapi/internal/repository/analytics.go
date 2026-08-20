package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type IAnalytics interface {
	GetDailyMetrics(ctx context.Context, from, to pgtype.Date) ([]db.GetDailyMetricsRow, error)
	GetLifecycleCounts(ctx context.Context, from, to pgtype.Timestamptz) ([]db.GetLifecycleCountsRow, error)
	GetLifecycleCountsByDay(ctx context.Context, from, to pgtype.Date) ([]db.GetLifecycleCountsByDayRow, error)
	GetConversionFunnelCounts(ctx context.Context, from, to pgtype.Timestamptz) ([]db.GetConversionFunnelCountsRow, error)
	GetRetentionCohorts(ctx context.Context, from, to pgtype.Date) ([]db.GetRetentionCohortsRow, error)
	GetActivationRate(ctx context.Context, from, to pgtype.Timestamptz) (db.GetActivationRateRow, error)
}

type analyticsRepo struct {
	db *db.Queries
}

func NewAnalyticsRepo(queries *db.Queries) IAnalytics {
	return &analyticsRepo{db: queries}
}

func (r *analyticsRepo) GetDailyMetrics(ctx context.Context, from, to pgtype.Date) ([]db.GetDailyMetricsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "AnalyticsRepo.GetDailyMetrics")
	defer span.End()
	return r.db.GetDailyMetrics(ctx, from, to)
}

func (r *analyticsRepo) GetLifecycleCounts(ctx context.Context, from, to pgtype.Timestamptz) ([]db.GetLifecycleCountsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "AnalyticsRepo.GetLifecycleCounts")
	defer span.End()
	return r.db.GetLifecycleCounts(ctx, from, to)
}

func (r *analyticsRepo) GetLifecycleCountsByDay(ctx context.Context, from, to pgtype.Date) ([]db.GetLifecycleCountsByDayRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "AnalyticsRepo.GetLifecycleCountsByDay")
	defer span.End()
	return r.db.GetLifecycleCountsByDay(ctx, from, to)
}

func (r *analyticsRepo) GetConversionFunnelCounts(ctx context.Context, from, to pgtype.Timestamptz) ([]db.GetConversionFunnelCountsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "AnalyticsRepo.GetConversionFunnelCounts")
	defer span.End()
	return r.db.GetConversionFunnelCounts(ctx, from, to)
}

func (r *analyticsRepo) GetRetentionCohorts(ctx context.Context, from, to pgtype.Date) ([]db.GetRetentionCohortsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "AnalyticsRepo.GetRetentionCohorts")
	defer span.End()
	return r.db.GetRetentionCohorts(ctx, from, to)
}

func (r *analyticsRepo) GetActivationRate(ctx context.Context, from, to pgtype.Timestamptz) (db.GetActivationRateRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "AnalyticsRepo.GetActivationRate")
	defer span.End()
	return r.db.GetActivationRate(ctx, from, to)
}
