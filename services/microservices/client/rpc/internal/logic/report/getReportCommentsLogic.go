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

type GetReportCommentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetReportCommentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportCommentsLogic {
	return &GetReportCommentsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetReportCommentsLogic) GetReportComments(in *client.GetReportCommentsRequest) (*client.GetReportCommentsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetReportCommentsLogic.GetReportComments")
	defer span.End()

	reportID, err := uuid.Parse(in.ReportId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reportId")
	}

	comments, err := l.svcCtx.Repo.Reports.ListReportComments(ctx, reportID)
	if err != nil {
		l.Errorf("failed to list report comments: %v", err)
		return nil, status.Error(codes.Internal, "failed to list comments")
	}

	pbComments := make([]*client.ReportComment, 0, len(comments))
	for _, c := range comments {
		pbComments = append(pbComments, commentToPb(c))
	}

	return &client.GetReportCommentsResponse{
		Comments: pbComments,
	}, nil
}
