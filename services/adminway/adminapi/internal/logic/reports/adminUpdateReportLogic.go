package reports

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientreport "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUpdateReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateReportLogic {
	return &AdminUpdateReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateReportLogic) AdminUpdateReport(req *types.UpdateReportRequest) (resp *types.ReportResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "report id is required")
	}
	if req.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	_, err = l.svcCtx.ReportRpc.UpdateReport(l.ctx, &clientreport.UpdateReportRequest{
		ReportId:   req.Id,
		Status:     req.Status,
		AdminNotes: req.AdminNotes,
	})
	if err != nil {
		return nil, err
	}

	// Re-fetch so the response reflects the persisted state.
	rpcResp, err := l.svcCtx.ReportRpc.GetReport(l.ctx, &clientreport.GetReportRequest{
		ReportId: req.Id,
	})
	if err != nil {
		return nil, err
	}
	if rpcResp.Report == nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	return &types.ReportResponse{
		Data: reportToType(rpcResp.Report),
	}, nil
}
