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

type CloseReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseReportLogic {
	return &CloseReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CloseReportLogic) CloseReport(in *client.CloseReportRequest) (*client.CloseReportResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CloseReportLogic.CloseReport")
	defer span.End()

	id, err := uuid.Parse(in.ReportId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reportId")
	}

	var closeReason *string
	if in.Reason != "" {
		r := in.Reason
		closeReason = &r
	}

	_, err = l.svcCtx.Repo.Reports.CloseReport(ctx, id, closeReason, nil)
	if err != nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	l.Infof("Closed report %s", in.ReportId)

	return &client.CloseReportResponse{
		Success: true,
	}, nil
}
