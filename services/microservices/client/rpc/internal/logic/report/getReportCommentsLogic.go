package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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

func (l *GetReportCommentsLogic) GetReportComments(in *client.GetReportCommentsRequest) (*client.GetReportCommentsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetReportCommentsLogic.GetReportComments")
	defer span.End()

	logx.WithContext(ctx).Infof("Getting comments for report %s", in.ReportId)

	return &client.GetReportCommentsResponse{
		Comments: []*client.ReportComment{},
	}, nil
}
