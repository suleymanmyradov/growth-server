package habittemplates

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetHabitTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetHabitTemplateLogic {
	return &AdminGetHabitTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetHabitTemplateLogic) AdminGetHabitTemplate(req *types.GetHabitTemplateRequest) (resp *types.HabitTemplateResponse, err error) {
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	row, err := l.svcCtx.Repo.HabitTemplates.Get(l.ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "habit template not found")
	}

	return &types.HabitTemplateResponse{
		Data: habitTemplateGetRowToItem(row),
	}, nil
}
