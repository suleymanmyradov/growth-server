package habittemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminCreateHabitTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCreateHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateHabitTemplateLogic {
	return &AdminCreateHabitTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateHabitTemplateLogic) AdminCreateHabitTemplate(req *types.CreateHabitTemplateRequest) (resp *types.HabitTemplateResponse, err error) {
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

	params := db.AdminCreateHabitTemplateParams{
		Name:        req.Name,
		Description: desc,
		CategoryID:  categoryID,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	}

	m, err := l.svcCtx.Repo.HabitTemplates.Create(l.ctx, params)
	if err != nil {
		return nil, err
	}

	return &types.HabitTemplateResponse{
		Data: habitTemplateModelToItem(m),
	}, nil
}
