package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/report/rpc/report"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetReportCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetReportCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportCategoriesLogic {
	return &GetReportCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Report metadata
func (l *GetReportCategoriesLogic) GetReportCategories(in *report.GetReportCategoriesRequest) (*report.GetReportCategoriesResponse, error) {
	// todo: add your logic here and delete this line

	return &report.GetReportCategoriesResponse{}, nil
}
