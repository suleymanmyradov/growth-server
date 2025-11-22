package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *GetReportCommentsLogic) GetReportComments(in *report.GetReportCommentsRequest) (*report.GetReportCommentsResponse, error) {
	// todo: add your logic here and delete this line

	return &report.GetReportCommentsResponse{}, nil
}
