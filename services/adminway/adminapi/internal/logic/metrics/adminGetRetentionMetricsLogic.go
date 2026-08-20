package metrics

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetRetentionMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetRetentionMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetRetentionMetricsLogic {
	return &AdminGetRetentionMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetRetentionMetricsLogic) AdminGetRetentionMetrics(req *types.MetricsRequest) (resp *types.RetentionMetricsResponse, err error) {
	from, to := parseDateRangeDates(req.FromDate, req.ToDate, 90)

	cohorts, err := l.svcCtx.Repo.Analytics.GetRetentionCohorts(l.ctx, from, to)
	if err != nil {
		l.Errorf("failed to get retention cohorts: %v", err)
		return nil, err
	}

	cohortList := make([]types.RetentionCohort, 0, len(cohorts))
	for _, c := range cohorts {
		rate := 0.0
		if c.RetentionRate.Valid {
			fv, _ := c.RetentionRate.Float64Value()
			rate = fv.Float64
		}
		cohortList = append(cohortList, types.RetentionCohort{
			CohortDate:    dateToString(c.CohortDate),
			CohortSize:    int64(c.CohortSize),
			PeriodDays:    c.PeriodDays,
			RetainedCount: int64(c.RetainedCount),
			RetentionRate: rate,
		})
	}

	return &types.RetentionMetricsResponse{
		Cohorts: cohortList,
	}, nil
}
