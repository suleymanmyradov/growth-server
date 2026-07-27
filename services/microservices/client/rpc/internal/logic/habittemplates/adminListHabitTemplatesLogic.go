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

type AdminListHabitTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminListHabitTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListHabitTemplatesLogic {
	return &AdminListHabitTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminListHabitTemplatesLogic) AdminListHabitTemplates(in *client.AdminListHabitTemplatesRequest) (*client.AdminListHabitTemplatesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminListHabitTemplatesLogic.AdminListHabitTemplates")
	defer span.End()

	templates, err := l.svcCtx.Repo.HabitTemplates.AdminListHabitTemplates(ctx)
	if err != nil {
		return nil, err
	}

	pbTemplates := make([]*client.HabitTemplate, len(templates))
	for i, t := range templates {
		pbTemplates[i] = convertAdminListHabitTemplateRow(t)
	}

	return &client.AdminListHabitTemplatesResponse{
		Templates: pbTemplates,
	}, nil
}

func convertAdminListHabitTemplateRow(row db.AdminListHabitTemplatesRow) *client.HabitTemplate {
	pb := &client.HabitTemplate{
		Id:        row.ID.String(),
		Name:      row.Name,
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
