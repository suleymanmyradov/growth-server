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

type AdminGetGoalTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminGetGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetGoalTemplateLogic {
	return &AdminGetGoalTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminGetGoalTemplateLogic) AdminGetGoalTemplate(in *client.AdminGetGoalTemplateRequest) (*client.GoalTemplate, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminGetGoalTemplateLogic.AdminGetGoalTemplate")
	defer span.End()

	if in == nil || in.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(in.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	row, err := l.svcCtx.Repo.GoalTemplates.AdminGetGoalTemplate(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get goal template")
	}

	return convertAdminGetGoalTemplateRow(row), nil
}

func convertAdminGetGoalTemplateRow(row db.AdminGetGoalTemplateRow) *client.GoalTemplate {
	pb := &client.GoalTemplate{
		Id:        row.ID.String(),
		Title:     row.Title,
		SortOrder: row.SortOrder,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time.Unix(),
		UpdatedAt: row.UpdatedAt.Time.Unix(),
	}
	if row.Description != nil {
		pb.Description = *row.Description
	}
	if row.CategoryIDJoined.Valid && row.CategoryIDJoined.UUID != uuid.Nil {
		name := ""
		if row.CategoryName != nil {
			name = *row.CategoryName
		}
		slug := ""
		if row.CategorySlug != nil {
			slug = *row.CategorySlug
		}
		pb.Category = &client.TemplateCategory{
			Id:   row.CategoryIDJoined.UUID.String(),
			Name: name,
			Slug: slug,
		}
	}
	return pb
}
