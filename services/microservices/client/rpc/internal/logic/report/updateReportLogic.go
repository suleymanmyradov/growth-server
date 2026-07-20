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

type UpdateReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateReportLogic {
	return &UpdateReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateReportLogic) UpdateReport(in *client.UpdateReportRequest) (*client.UpdateReportResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateReportLogic.UpdateReport")
	defer span.End()

	id, err := uuid.Parse(in.ReportId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reportId")
	}
	if in.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	var adminNotes *string
	if in.AdminNotes != "" {
		n := in.AdminNotes
		adminNotes = &n
	}

	report, err := l.svcCtx.Repo.Reports.UpdateReportStatus(ctx, id, in.Status, adminNotes)
	if err != nil {
		return nil, status.Error(codes.NotFound, "report not found")
	}

	l.Infof("Updated report %s status=%s", in.ReportId, in.Status)
	_ = report

	return &client.UpdateReportResponse{
		Success: true,
	}, nil
}
