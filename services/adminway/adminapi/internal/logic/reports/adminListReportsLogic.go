package reports

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientreport "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListReportsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListReportsLogic {
	return &AdminListReportsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListReportsLogic) AdminListReports(req *types.ListReportsRequest) (resp *types.ReportsResponse, err error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	rpcResp, err := l.svcCtx.ReportRpc.ListReports(l.ctx, &clientreport.ListReportsRequest{
		Status:   req.Status,
		Category: req.Category,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.ReportItem, 0, len(rpcResp.Reports))
	for _, r := range rpcResp.Reports {
		items = append(items, reportToType(r))
	}

	return &types.ReportsResponse{
		Data: items,
		Page: types.PageResponse{
			Total:      int64(rpcResp.TotalCount),
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages(int(rpcResp.TotalCount), limit),
		},
	}, nil
}
