package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

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

// Comments
func (l *AddReportCommentLogic) AddReportComment(in *report.AddReportCommentRequest) (*report.AddReportCommentResponse, error) {
	// todo: add your logic here and delete this line

	return &report.AddReportCommentResponse{}, nil
}
