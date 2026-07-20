package reports

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientreport "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminAddReportCommentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAddReportCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAddReportCommentLogic {
	return &AdminAddReportCommentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminAddReportCommentLogic) AdminAddReportComment(req *types.AddReportCommentRequest) (resp *types.AddReportCommentResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "report id is required")
	}
	if req.Comment == "" {
		return nil, status.Error(codes.InvalidArgument, "comment is required")
	}

	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "admin identity required")
	}

	rpcResp, err := l.svcCtx.ReportRpc.AddReportComment(l.ctx, &clientreport.AddReportCommentRequest{
		ReportId: req.Id,
		UserId:   p.UserID,
		Comment:  req.Comment,
		IsAdmin:  true,
	})
	if err != nil {
		return nil, err
	}

	// Re-fetch the single comment so we can return the full row (createdAt, etc.).
	// The AddReportComment RPC only returns the comment id, so we list and pick.
	listResp, err := l.svcCtx.ReportRpc.GetReportComments(l.ctx, &clientreport.GetReportCommentsRequest{
		ReportId: req.Id,
	})
	if err != nil {
		// Fallback: return a minimal item with just the id we got back.
		return &types.AddReportCommentResponse{
			Data: types.ReportCommentItem{
				Id:       rpcResp.CommentId,
				ReportId: req.Id,
				UserId:   p.UserID,
				Comment:  req.Comment,
				IsAdmin:  true,
			},
		}, nil
	}

	for _, c := range listResp.Comments {
		if c.Id == rpcResp.CommentId {
			return &types.AddReportCommentResponse{
				Data: commentToType(c),
			}, nil
		}
	}

	return &types.AddReportCommentResponse{
		Data: types.ReportCommentItem{
			Id:       rpcResp.CommentId,
			ReportId: req.Id,
			UserId:   p.UserID,
			Comment:  req.Comment,
			IsAdmin:  true,
		},
	}, nil
}
