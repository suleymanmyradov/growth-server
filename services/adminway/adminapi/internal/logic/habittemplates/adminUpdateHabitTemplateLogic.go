package habittemplates

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

type AdminUpdateHabitTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateHabitTemplateLogic {
	return &AdminUpdateHabitTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateHabitTemplateLogic) AdminUpdateHabitTemplate(req *types.UpdateHabitTemplateRequest) (resp *types.HabitTemplateResponse, err error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
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

	params := db.AdminUpdateHabitTemplateParams{
		ID:          id,
		Name:        req.Name,
		Description: desc,
		CategoryID:  categoryID,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	}

	m, err := l.svcCtx.Repo.HabitTemplates.Update(l.ctx, params)
	if err != nil {
		return nil, status.Error(codes.NotFound, "habit template not found")
	}

	return &types.HabitTemplateResponse{
		Data: habitTemplateModelToItem(m),
	}, nil
}
