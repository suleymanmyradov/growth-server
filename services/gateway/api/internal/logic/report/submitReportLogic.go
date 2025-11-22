// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package report

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubmitReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubmitReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitReportLogic {
	return &SubmitReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubmitReportLogic) SubmitReport(req *types.ReportRequest) (resp *types.EmptyResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
