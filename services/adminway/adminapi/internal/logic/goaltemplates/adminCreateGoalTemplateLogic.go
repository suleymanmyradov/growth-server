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

type AdminCreateGoalTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCreateGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateGoalTemplateLogic {
	return &AdminCreateGoalTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateGoalTemplateLogic) AdminCreateGoalTemplate(req *types.CreateGoalTemplateRequest) (resp *types.GoalTemplateResponse, err error) {
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	categoryId := ""
	if req.CategoryId != nil {
		categoryId = *req.CategoryId
	}

	t, err := l.svcCtx.GoalTemplatesRpc.AdminCreateGoalTemplate(l.ctx, &clientgoaltemplates.AdminCreateGoalTemplateRequest{
		Title:       req.Title,
		Description: req.Description,
		CategoryId:  categoryId,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalTemplateResponse{
		Data: goalTemplateProtoToItem(t),
	}, nil
}
