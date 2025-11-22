package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *CloseReportLogic) CloseReport(in *report.CloseReportRequest) (*report.CloseReportResponse, error) {
	// todo: add your logic here and delete this line

	return &report.CloseReportResponse{}, nil
}
