package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

	"github.com/zeromicro/go-zero/core/logx"
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

// Report CRUD
func (l *SubmitReportLogic) SubmitReport(in *report.SubmitReportRequest) (*report.SubmitReportResponse, error) {
	// todo: add your logic here and delete this line

	return &report.SubmitReportResponse{}, nil
}
