package reports

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientreport "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListReportCommentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListReportCommentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListReportCommentsLogic {
	return &AdminListReportCommentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListReportCommentsLogic) AdminListReportComments(req *types.ReportIdRequest) (resp *types.ReportCommentsResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "report id is required")
	}

	rpcResp, err := l.svcCtx.ReportRpc.GetReportComments(l.ctx, &clientreport.GetReportCommentsRequest{
		ReportId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.ReportCommentItem, 0, len(rpcResp.Comments))
	for _, c := range rpcResp.Comments {
		items = append(items, commentToType(c))
	}

	return &types.ReportCommentsResponse{
		Data: items,
	}, nil
}
