package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListReportsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReportsLogic {
	return &ListReportsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListReportsLogic) ListReports(in *report.ListReportsRequest) (*report.ListReportsResponse, error) {
	// todo: add your logic here and delete this line

	return &report.ListReportsResponse{}, nil
}
