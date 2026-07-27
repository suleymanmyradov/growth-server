package goaltemplateslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type AdminListGoalTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminListGoalTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListGoalTemplatesLogic {
	return &AdminListGoalTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminListGoalTemplatesLogic) AdminListGoalTemplates(in *client.AdminListGoalTemplatesRequest) (*client.AdminListGoalTemplatesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminListGoalTemplatesLogic.AdminListGoalTemplates")
	defer span.End()

	templates, err := l.svcCtx.Repo.GoalTemplates.AdminListGoalTemplates(ctx)
	if err != nil {
		return nil, err
	}

	pbTemplates := make([]*client.GoalTemplate, len(templates))
	for i, t := range templates {
		pbTemplates[i] = convertAdminListGoalTemplateRow(t)
	}

	return &client.AdminListGoalTemplatesResponse{
		Templates: pbTemplates,
	}, nil
}

func convertAdminListGoalTemplateRow(row db.AdminListGoalTemplatesRow) *client.GoalTemplate {
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
