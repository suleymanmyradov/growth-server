package metrics

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetMonetizationMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetMonetizationMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetMonetizationMetricsLogic {
	return &AdminGetMonetizationMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetMonetizationMetricsLogic) AdminGetMonetizationMetrics(req *types.MetricsRequest) (resp *types.MonetizationMetricsResponse, err error) {
	from, to := parseDateRange(req.FromDate, req.ToDate, 90)

	funnelCounts, err := l.svcCtx.Repo.Analytics.GetConversionFunnelCounts(l.ctx, from, to)
	if err != nil {
		l.Errorf("failed to get conversion funnel counts: %v", err)
		return nil, err
	}

	funnel := make([]types.ConversionStage, 0, len(funnelCounts))
	for _, fc := range funnelCounts {
		funnel = append(funnel, types.ConversionStage{
			Stage: fc.Stage,
			Count: fc.Count,
		})
	}

	return &types.MonetizationMetricsResponse{
		Funnel: funnel,
	}, nil
}
