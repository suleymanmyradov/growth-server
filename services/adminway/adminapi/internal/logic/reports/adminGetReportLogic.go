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

type AdminGetReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetReportLogic {
	return &AdminGetReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetReportLogic) AdminGetReport(req *types.ReportIdRequest) (resp *types.ReportResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "report id is required")
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
