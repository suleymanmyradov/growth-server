package metrics

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetBehaviorMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetBehaviorMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetBehaviorMetricsLogic {
	return &AdminGetBehaviorMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetBehaviorMetricsLogic) AdminGetBehaviorMetrics(req *types.MetricsRequest) (resp *types.BehaviorMetricsResponse, err error) {
	from, to := parseDateRangeDates(req.FromDate, req.ToDate, 30)

	metrics, err := l.svcCtx.Repo.Analytics.GetDailyMetrics(l.ctx, from, to)
	if err != nil {
		l.Errorf("failed to get daily metrics: %v", err)
		return nil, err
	}

	metricList := make([]types.DailyMetric, 0, len(metrics))
	for _, m := range metrics {
		metricList = append(metricList, types.DailyMetric{
			Date:        dateToString(m.MetricDate),
			MetricName:  m.MetricName,
			MetricValue: int64(m.MetricValue),
		})
	}

	return &types.BehaviorMetricsResponse{
		Metrics: metricList,
	}, nil
}
