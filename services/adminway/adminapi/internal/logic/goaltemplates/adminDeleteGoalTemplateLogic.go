package goaltemplates

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDeleteGoalTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDeleteGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteGoalTemplateLogic {
	return &AdminDeleteGoalTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteGoalTemplateLogic) AdminDeleteGoalTemplate(req *types.DeleteGoalTemplateRequest) (resp *types.EmptyResponse, err error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if err := l.svcCtx.Repo.GoalTemplates.Delete(l.ctx, id); err != nil {
		return nil, status.Error(codes.NotFound, "goal template not found")
	}

	return &types.EmptyResponse{}, nil
}
