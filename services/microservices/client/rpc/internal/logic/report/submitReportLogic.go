package reportlogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type SubmitReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitReportLogic {
	return &SubmitReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SubmitReportLogic) SubmitReport(in *client.SubmitReportRequest) (*client.SubmitReportResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "SubmitReportLogic.SubmitReport")
	defer span.End()

	reporterID, err := uuid.Parse(in.ReporterId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reporterId")
	}
	if in.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	if in.Description == "" {
		return nil, status.Error(codes.InvalidArgument, "description is required")
	}
	if in.Category == "" {
		return nil, status.Error(codes.InvalidArgument, "category is required")
	}

	var targetID *string
	if in.TargetId != "" {
		t := in.TargetId
		targetID = &t
	}
	var email *string
	if in.Email != "" {
		e := in.Email
		email = &e
	}
	targetType := in.TargetType
	if targetType == "" {
		targetType = "general"
	}
	attachments := in.Attachments
	if attachments == nil {
		attachments = []string{}
	}

	report, err := l.svcCtx.Repo.Reports.CreateReport(ctx, db.CreateReportParams{
		ReporterID:  reporterID,
		TargetID:    targetID,
		TargetType:  targetType,
		Category:    in.Category,
		Title:       in.Title,
		Description: in.Description,
		Email:       email,
		Attachments: attachments,
	})
	if err != nil {
		l.Errorf("failed to create report: %v", err)
		return nil, status.Error(codes.Internal, "failed to submit report")
	}

	l.Infof("Submitted report %s from user %s (category=%s)", report.ID, in.ReporterId, in.Category)

	return &client.SubmitReportResponse{
		ReportId: report.ID.String(),
	}, nil
}
