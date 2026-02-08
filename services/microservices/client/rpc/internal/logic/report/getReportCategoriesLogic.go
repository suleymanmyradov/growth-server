package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *GetReportCategoriesLogic) GetReportCategories(in *client.GetReportCategoriesRequest) (*client.GetReportCategoriesResponse, error) {
	l.Logger.Infof("Getting report categories")

	return &client.GetReportCategoriesResponse{
		Categories: []*client.ReportCategory{},
	}, nil
}
