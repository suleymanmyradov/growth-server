package reportlogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type GetReportStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetReportStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportStatusLogic {
	return &GetReportStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetReportStatusLogic) GetReportStatus(in *client.GetReportStatusRequest) (*client.GetReportStatusResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetReportStatusLogic.GetReportStatus")
	defer span.End()

	id, err := uuid.Parse(in.ReportId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reportId")
	}

	row, err := l.svcCtx.Repo.Reports.GetReportStatus(ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	return &client.GetReportStatusResponse{
		Status:    row.Status,
		UpdatedAt: row.UpdatedAt.Time.Unix(),
	}, nil
}
