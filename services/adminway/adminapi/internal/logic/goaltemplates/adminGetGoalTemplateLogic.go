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

type AdminGetGoalTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetGoalTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetGoalTemplateLogic {
	return &AdminGetGoalTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetGoalTemplateLogic) AdminGetGoalTemplate(req *types.GetGoalTemplateRequest) (resp *types.GoalTemplateResponse, err error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	row, err := l.svcCtx.Repo.GoalTemplates.Get(l.ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "goal template not found")
	}

	return &types.GoalTemplateResponse{
		Data: goalTemplateGetRowToItem(row),
	}, nil
}
