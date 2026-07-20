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

type ListGoalTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListGoalTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGoalTemplatesLogic {
	return &ListGoalTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListGoalTemplatesLogic) ListGoalTemplates(in *client.ListGoalTemplatesRequest) (*client.ListGoalTemplatesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListGoalTemplatesLogic.ListGoalTemplates")
	defer span.End()

	templates, err := l.svcCtx.Repo.GoalTemplates.ListGoalTemplates(ctx)
	if err != nil {
		return nil, err
	}

	pbTemplates := make([]*client.GoalTemplate, len(templates))
	for i, t := range templates {
		pbTemplates[i] = convertGoalTemplate(t)
	}

	return &client.ListGoalTemplatesResponse{
		Templates: pbTemplates,
	}, nil
}

func convertGoalTemplate(t db.ListGoalTemplatesRow) *client.GoalTemplate {
	pb := &client.GoalTemplate{
		Id:          t.ID.String(),
		Title:       t.Title,
		SortOrder:   t.SortOrder,
		CreatedAt:   t.CreatedAt.Time.Unix(),
		UpdatedAt:   t.UpdatedAt.Time.Unix(),
	}
	if t.Description != nil {
		pb.Description = *t.Description
	}
	if t.CategoryID.Valid && t.CategoryID.UUID != uuid.Nil {
		name := ""
		if t.CategoryName != nil {
			name = *t.CategoryName
		}
		slug := ""
		if t.CategorySlug != nil {
			slug = *t.CategorySlug
		}
		pb.Category = &client.TemplateCategory{
			Id:   t.CategoryID.UUID.String(),
			Name: name,
			Slug: slug,
		}
	}
	return pb
}
