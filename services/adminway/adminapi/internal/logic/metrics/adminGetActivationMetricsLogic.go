package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetActivationMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetActivationMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetActivationMetricsLogic {
	return &AdminGetActivationMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetActivationMetricsLogic) AdminGetActivationMetrics(req *types.MetricsRequest) (resp *types.ActivationMetricsResponse, err error) {
	from, to := parseDateRange(req.FromDate, req.ToDate, 30)

	activation, err := l.svcCtx.Repo.Analytics.GetActivationRate(l.ctx, from, to)
	if err != nil {
		l.Errorf("failed to get activation rate: %v", err)
		return nil, err
	}

	lifecycleCounts, err := l.svcCtx.Repo.Analytics.GetLifecycleCounts(l.ctx, from, to)
	if err != nil {
		l.Errorf("failed to get lifecycle counts: %v", err)
		return nil, err
	}

	lifecycle := make([]types.LifecycleCount, 0, len(lifecycleCounts))
	for _, lc := range lifecycleCounts {
		lifecycle = append(lifecycle, types.LifecycleCount{
			EventType: lc.EventType,
			Count:     lc.Count,
		})
	}

	rate := 0.0
	if activation.TotalOnboarded > 0 {
		rate = float64(activation.Activated) / float64(activation.TotalOnboarded)
	}

	return &types.ActivationMetricsResponse{
		Activated:      activation.Activated,
		TotalOnboarded: activation.TotalOnboarded,
		ActivationRate: rate,
		Lifecycle:      lifecycle,
	}, nil
}

// parseDateRange converts optional YYYY-MM-DD strings into pgtype timestamps.
// If both are empty, defaults to the last N days.
func parseDateRange(fromStr, toStr string, defaultDays int) (pgtype.Timestamptz, pgtype.Timestamptz) {
	var from, to pgtype.Timestamptz
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = pgtype.Timestamptz{Time: t.Add(24 * time.Hour), Valid: true}
		}
	}
	if !from.Valid && !to.Valid {
		now := time.Now().UTC()
		from = pgtype.Timestamptz{Time: now.AddDate(0, 0, -defaultDays), Valid: true}
		to = pgtype.Timestamptz{Time: now, Valid: true}
	}
	return from, to
}

// parseDateRangeDates converts optional YYYY-MM-DD strings into pgtype dates.
func parseDateRangeDates(fromStr, toStr string, defaultDays int) (pgtype.Date, pgtype.Date) {
	var from, to pgtype.Date
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = pgtype.Date{Time: t, Valid: true}
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = pgtype.Date{Time: t, Valid: true}
		}
	}
	if !from.Valid && !to.Valid {
		now := time.Now().UTC()
		from = pgtype.Date{Time: now.AddDate(0, 0, -defaultDays), Valid: true}
		to = pgtype.Date{Time: now, Valid: true}
	}
	return from, to
}

// dateToString converts a pgtype.Date to a YYYY-MM-DD string, or "" if invalid.
func dateToString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}
