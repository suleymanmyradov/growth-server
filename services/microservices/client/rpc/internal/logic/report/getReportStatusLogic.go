package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *GetReportStatusLogic) GetReportStatus(in *client.GetReportStatusRequest) (*client.GetReportStatusResponse, error) {
	l.Logger.Infof("Getting status for report %s", in.ReportId)

	return &client.GetReportStatusResponse{
		Status: "",
	}, nil
}
