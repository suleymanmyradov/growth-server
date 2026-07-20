package goaltemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
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

	categoryID, err := parseOptionalUUID(req.CategoryId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid categoryId")
	}

	var desc *string
	if req.Description != "" {
		d := req.Description
		desc = &d
	}

	params := db.AdminCreateGoalTemplateParams{
		Title:       req.Title,
		Description: desc,
		CategoryID:  categoryID,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	}

	m, err := l.svcCtx.Repo.GoalTemplates.Create(l.ctx, params)
	if err != nil {
		return nil, err
	}

	return &types.GoalTemplateResponse{
		Data: goalTemplateModelToItem(m),
	}, nil
}
