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

type AdminUpdateHabitTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminUpdateHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateHabitTemplateLogic {
	return &AdminUpdateHabitTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminUpdateHabitTemplateLogic) AdminUpdateHabitTemplate(in *client.AdminUpdateHabitTemplateRequest) (*client.HabitTemplate, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminUpdateHabitTemplateLogic.AdminUpdateHabitTemplate")
	defer span.End()

	if in == nil || in.Id == "" || in.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "id and name are required")
	}

	id, err := uuid.Parse(in.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	params := db.AdminUpdateHabitTemplateParams{
		ID:        id,
		Name:      in.Name,
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

	t, err := l.svcCtx.Repo.HabitTemplates.AdminUpdateHabitTemplate(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update habit template")
	}

	return &client.HabitTemplate{
		Id:        t.ID.String(),
		Name:      t.Name,
		SortOrder: t.SortOrder,
		IsActive:  t.IsActive,
		CreatedAt: t.CreatedAt.Time.Unix(),
		UpdatedAt: t.UpdatedAt.Time.Unix(),
	}, nil
}
