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

type AdminCreateHabitTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminCreateHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateHabitTemplateLogic {
	return &AdminCreateHabitTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminCreateHabitTemplateLogic) AdminCreateHabitTemplate(in *client.AdminCreateHabitTemplateRequest) (*client.HabitTemplate, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminCreateHabitTemplateLogic.AdminCreateHabitTemplate")
	defer span.End()

	if in == nil || in.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	params := db.AdminCreateHabitTemplateParams{
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

	t, err := l.svcCtx.Repo.HabitTemplates.AdminCreateHabitTemplate(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create habit template")
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
