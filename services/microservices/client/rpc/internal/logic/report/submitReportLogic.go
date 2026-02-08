package reportlogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubmitReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitReportLogic {
	return &SubmitReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SubmitReportLogic) SubmitReport(in *client.SubmitReportRequest) (*client.SubmitReportResponse, error) {
	l.Logger.Infof("Submitting report from user %s for target %s", in.ReporterId, in.TargetId)

	return &client.SubmitReportResponse{
		ReportId: uuid.New().String(),
	}, nil
}
