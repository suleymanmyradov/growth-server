package goaltemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientgoaltemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goaltemplates"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminUpdateGoalTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateGoalTemplateLogic {
	return &AdminUpdateGoalTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateGoalTemplateLogic) AdminUpdateGoalTemplate(req *types.UpdateGoalTemplateRequest) (resp *types.GoalTemplateResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	categoryId := ""
	if req.CategoryId != nil {
		categoryId = *req.CategoryId
	}

	t, err := l.svcCtx.GoalTemplatesRpc.AdminUpdateGoalTemplate(l.ctx, &clientgoaltemplates.AdminUpdateGoalTemplateRequest{
		Id:          req.Id,
		Title:       req.Title,
		Description: req.Description,
		CategoryId:  categoryId,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "goal template not found")
	}

	return &types.GoalTemplateResponse{
		Data: goalTemplateProtoToItem(t),
	}, nil
}
