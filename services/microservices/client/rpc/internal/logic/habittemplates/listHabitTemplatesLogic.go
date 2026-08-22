package habittemplateslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ListHabitTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListHabitTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHabitTemplatesLogic {
	return &ListHabitTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListHabitTemplatesLogic) ListHabitTemplates(in *client.ListHabitTemplatesRequest) (*client.ListHabitTemplatesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListHabitTemplatesLogic.ListHabitTemplates")
	defer span.End()

	templates, err := l.svcCtx.Repo.HabitTemplates.ListHabitTemplates(ctx)
	if err != nil {
		return nil, err
	}

	pbTemplates := make([]*client.HabitTemplate, len(templates))
	for i, t := range templates {
		pbTemplates[i] = convertHabitTemplate(t)
	}

	return &client.ListHabitTemplatesResponse{
		Templates: pbTemplates,
	}, nil
}

func convertHabitTemplate(t db.ListHabitTemplatesRow) *client.HabitTemplate {
	pb := &client.HabitTemplate{
		Id:        t.ID.String(),
		Name:      t.Name,
		SortOrder: t.SortOrder,
		CreatedAt: t.CreatedAt.Time.Unix(),
		UpdatedAt: t.UpdatedAt.Time.Unix(),
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
