package reportlogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type GetReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportLogic {
	return &GetReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetReportLogic) GetReport(in *client.GetReportRequest) (*client.GetReportResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetReportLogic.GetReport")
	defer span.End()

	id, err := uuid.Parse(in.ReportId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reportId")
	}

	report, err := l.svcCtx.Repo.Reports.GetReportByID(ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	return &client.GetReportResponse{
		Report: reportToPb(report),
	}, nil
}
