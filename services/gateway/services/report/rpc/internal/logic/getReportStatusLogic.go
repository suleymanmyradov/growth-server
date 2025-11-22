package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetReportStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetReportStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportStatusLogic {
	return &GetReportStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetReportStatusLogic) GetReportStatus(in *report.GetReportStatusRequest) (*report.GetReportStatusResponse, error) {
	// todo: add your logic here and delete this line

	return &report.GetReportStatusResponse{}, nil
}
