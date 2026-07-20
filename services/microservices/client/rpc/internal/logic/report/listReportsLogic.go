package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ListReportsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReportsLogic {
	return &ListReportsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListReportsLogic) ListReports(in *client.ListReportsRequest) (*client.ListReportsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListReportsLogic.ListReports")
	defer span.End()

	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	reporterID := parseReporterID(in.ReporterId)

	reports, err := l.svcCtx.Repo.Reports.ListReports(ctx, db.ListReportsParams{
		Limit:      limit,
		Offset:     offset,
		Status:     in.Status,
		Category:   in.Category,
		ReporterID: reporterID,
	})
	if err != nil {
		l.Errorf("failed to list reports: %v", err)
		return nil, status.Error(codes.Internal, "failed to list reports")
	}

	pbReports := make([]*client.ReportItem, 0, len(reports))
	for _, r := range reports {
		pbReports = append(pbReports, reportToPb(r))
	}

	totalCount, _ := l.svcCtx.Repo.Reports.CountReports(ctx, in.Status, in.Category, reporterID)

	return &client.ListReportsResponse{
		Reports:    pbReports,
		TotalCount: int32(totalCount),
	}, nil
}
