package goaltemplates

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
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
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

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

	params := db.AdminUpdateGoalTemplateParams{
		ID:          id,
		Title:       req.Title,
		Description: desc,
		CategoryID:  categoryID,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	}

	m, err := l.svcCtx.Repo.GoalTemplates.Update(l.ctx, params)
	if err != nil {
		return nil, status.Error(codes.NotFound, "goal template not found")
	}

	return &types.GoalTemplateResponse{
		Data: goalTemplateModelToItem(m),
	}, nil
}
