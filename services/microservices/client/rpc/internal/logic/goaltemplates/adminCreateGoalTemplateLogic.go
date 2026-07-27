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

type AdminCreateGoalTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminCreateGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateGoalTemplateLogic {
	return &AdminCreateGoalTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminCreateGoalTemplateLogic) AdminCreateGoalTemplate(in *client.AdminCreateGoalTemplateRequest) (*client.GoalTemplate, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminCreateGoalTemplateLogic.AdminCreateGoalTemplate")
	defer span.End()

	if in == nil || in.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	params := db.AdminCreateGoalTemplateParams{
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

	t, err := l.svcCtx.Repo.GoalTemplates.AdminCreateGoalTemplate(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create goal template")
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
