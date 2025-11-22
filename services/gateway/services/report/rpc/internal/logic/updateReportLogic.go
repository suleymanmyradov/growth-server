package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *UpdateReportLogic) UpdateReport(in *report.UpdateReportRequest) (*report.UpdateReportResponse, error) {
	// todo: add your logic here and delete this line

	return &report.UpdateReportResponse{}, nil
}
