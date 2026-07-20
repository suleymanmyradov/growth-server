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

type AdminCloseReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCloseReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCloseReportLogic {
	return &AdminCloseReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCloseReportLogic) AdminCloseReport(req *types.CloseReportRequest) (resp *types.ReportResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "report id is required")
	}

	_, err = l.svcCtx.ReportRpc.CloseReport(l.ctx, &clientreport.CloseReportRequest{
		ReportId: req.Id,
		Reason:   req.Reason,
	})
	if err != nil {
		return nil, err
	}

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
