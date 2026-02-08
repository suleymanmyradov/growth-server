package reportlogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *ListReportsLogic) ListReports(in *client.ListReportsRequest) (*client.ListReportsResponse, error) {
	l.Logger.Infof("Listing reports for reporter %s", in.ReporterId)

	return &client.ListReportsResponse{
		Reports:    []*client.ReportItem{},
		TotalCount: 0,
	}, nil
}
