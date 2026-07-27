package habittemplateslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AdminDeleteHabitTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminDeleteHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteHabitTemplateLogic {
	return &AdminDeleteHabitTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminDeleteHabitTemplateLogic) AdminDeleteHabitTemplate(in *client.AdminDeleteHabitTemplateRequest) (*client.AdminDeleteHabitTemplateResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminDeleteHabitTemplateLogic.AdminDeleteHabitTemplate")
	defer span.End()

	if in == nil || in.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(in.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if err := l.svcCtx.Repo.HabitTemplates.AdminDeleteHabitTemplate(ctx, id); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete habit template")
	}

	return &client.AdminDeleteHabitTemplateResponse{Success: true}, nil
}
