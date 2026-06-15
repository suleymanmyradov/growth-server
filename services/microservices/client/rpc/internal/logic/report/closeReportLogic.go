package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type CloseReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseReportLogic {
	return &CloseReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CloseReportLogic) CloseReport(in *client.CloseReportRequest) (*client.CloseReportResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CloseReportLogic.CloseReport")
	defer span.End()

	logx.WithContext(ctx).Infof("Closing report %s", in.ReportId)

	return &client.CloseReportResponse{
		Success: true,
	}, nil
}
