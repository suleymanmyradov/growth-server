package habittemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clienthabittemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habittemplates"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDeleteHabitTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDeleteHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteHabitTemplateLogic {
	return &AdminDeleteHabitTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteHabitTemplateLogic) AdminDeleteHabitTemplate(req *types.DeleteHabitTemplateRequest) (resp *types.EmptyResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if _, err := l.svcCtx.HabitTemplatesRpc.AdminDeleteHabitTemplate(l.ctx, &clienthabittemplates.AdminDeleteHabitTemplateRequest{
		Id: req.Id,
	}); err != nil {
		return nil, status.Error(codes.NotFound, "habit template not found")
	}

	return &types.EmptyResponse{}, nil
}
