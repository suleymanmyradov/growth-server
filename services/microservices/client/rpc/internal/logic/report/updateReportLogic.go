package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *UpdateReportLogic) UpdateReport(in *client.UpdateReportRequest) (*client.UpdateReportResponse, error) {
	l.Logger.Infof("Updating report %s", in.ReportId)

	return &client.UpdateReportResponse{
		Success: true,
	}, nil
}
