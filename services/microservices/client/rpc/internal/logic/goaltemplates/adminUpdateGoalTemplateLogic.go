package goaltemplateslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdminUpdateGoalTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminUpdateGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateGoalTemplateLogic {
	return &AdminUpdateGoalTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminUpdateGoalTemplateLogic) AdminUpdateGoalTemplate(in *client.AdminUpdateGoalTemplateRequest) (*client.GoalTemplate, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminUpdateGoalTemplateLogic.AdminUpdateGoalTemplate")
	defer span.End()

	if in == nil || in.Id == "" || in.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "id and title are required")
	}

	id, err := uuid.Parse(in.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	params := db.AdminUpdateGoalTemplateParams{
		ID:        id,
		Title:     in.Title,
		SortOrder: in.SortOrder,
		IsActive:  in.IsActive,
	}
	if in.Description != "" {
		params.Description = &in.Description
	}
	if in.CategoryId != "" {
		catID, err := uuid.Parse(in.CategoryId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid categoryId")
		}
		params.CategoryID = uuid.NullUUID{UUID: catID, Valid: true}
	}

	t, err := l.svcCtx.Repo.GoalTemplates.AdminUpdateGoalTemplate(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update goal template")
	}

	return &client.GoalTemplate{
		Id:        t.ID.String(),
		Title:     t.Title,
		SortOrder: t.SortOrder,
		IsActive:  t.IsActive,
		CreatedAt: t.CreatedAt.Time.Unix(),
		UpdatedAt: t.UpdatedAt.Time.Unix(),
	}, nil
}
