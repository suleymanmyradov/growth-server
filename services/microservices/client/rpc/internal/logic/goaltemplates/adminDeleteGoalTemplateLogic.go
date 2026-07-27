package goaltemplateslogic

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

type AdminDeleteGoalTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminDeleteGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteGoalTemplateLogic {
	return &AdminDeleteGoalTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminDeleteGoalTemplateLogic) AdminDeleteGoalTemplate(in *client.AdminDeleteGoalTemplateRequest) (*client.AdminDeleteGoalTemplateResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminDeleteGoalTemplateLogic.AdminDeleteGoalTemplate")
	defer span.End()

	if in == nil || in.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	id, err := uuid.Parse(in.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if err := l.svcCtx.Repo.GoalTemplates.AdminDeleteGoalTemplate(ctx, id); err != nil {
		return nil, status.Error(codes.Internal, "failed to delete goal template")
	}

	return &client.AdminDeleteGoalTemplateResponse{Success: true}, nil
}
