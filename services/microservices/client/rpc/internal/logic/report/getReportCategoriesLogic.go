package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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

// reportCategories is the fixed set of report categories. The frontend maps
// its UI labels onto these slugs (bug, feedback, abuse).
var reportCategories = []*client.ReportCategory{
	{Id: "bug", Name: "Bug", Description: "Something isn't working as expected"},
	{Id: "feedback", Name: "Feedback", Description: "A suggestion or idea to improve the app"},
	{Id: "abuse", Name: "Abuse / Spam", Description: "Inappropriate or abusive content"},
}

func (l *GetReportCategoriesLogic) GetReportCategories(_ *client.GetReportCategoriesRequest) (*client.GetReportCategoriesResponse, error) {
	_, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetReportCategoriesLogic.GetReportCategories")
	defer span.End()

	return &client.GetReportCategoriesResponse{
		Categories: reportCategories,
	}, nil
}
