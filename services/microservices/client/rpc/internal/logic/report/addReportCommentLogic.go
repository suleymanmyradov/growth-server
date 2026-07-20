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

type AddReportCommentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddReportCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddReportCommentLogic {
	return &AddReportCommentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddReportCommentLogic) AddReportComment(in *client.AddReportCommentRequest) (*client.AddReportCommentResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AddReportCommentLogic.AddReportComment")
	defer span.End()

	reportID, err := uuid.Parse(in.ReportId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid reportId")
	}
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid userId")
	}
	if in.Comment == "" {
		return nil, status.Error(codes.InvalidArgument, "comment is required")
	}

	comment, err := l.svcCtx.Repo.Reports.CreateReportComment(ctx, reportID, userID, in.Comment, in.IsAdmin)
	if err != nil {
		l.Errorf("failed to add report comment: %v", err)
		return nil, status.Error(codes.Internal, "failed to add comment")
	}

	return &client.AddReportCommentResponse{
		CommentId: comment.ID.String(),
	}, nil
}
