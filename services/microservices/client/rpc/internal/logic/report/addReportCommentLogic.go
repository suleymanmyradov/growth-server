package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	l.Logger.Infof("Adding comment to report %s", in.ReportId)

	return &client.AddReportCommentResponse{
		CommentId: "",
	}, nil
}
