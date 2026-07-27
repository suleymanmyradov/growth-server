package habittemplateslogic

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

type AdminGetHabitTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminGetHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetHabitTemplateLogic {
	return &AdminGetHabitTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminGetHabitTemplateLogic) AdminGetHabitTemplate(in *client.AdminGetHabitTemplateRequest) (*client.HabitTemplate, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminGetHabitTemplateLogic.AdminGetHabitTemplate")
	defer span.End()

	if in == nil || in.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(in.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	row, err := l.svcCtx.Repo.HabitTemplates.AdminGetHabitTemplate(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get habit template")
	}

	return convertAdminHabitTemplateRow(row), nil
}

// convertAdminHabitTemplateRow converts an admin query row (with joined
// category fields) to a proto HabitTemplate.
func convertAdminHabitTemplateRow(row db.AdminGetHabitTemplateRow) *client.HabitTemplate {
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
