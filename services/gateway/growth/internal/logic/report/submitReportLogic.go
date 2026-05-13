// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package report

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientreport "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.EmptyResponse{}, nil
	}

	_, err = l.svcCtx.ReportRpc.SubmitReport(l.ctx, &clientreport.SubmitReportRequest{
		ReporterId:  p.UserID,
		TargetType:  req.ItemType,
		Category:    req.ItemType,
		Description: req.Description,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
