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
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	t, err := l.svcCtx.HabitTemplatesRpc.AdminGetHabitTemplate(l.ctx, &clienthabittemplates.AdminGetHabitTemplateRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "habit template not found")
	}

	return &types.HabitTemplateResponse{
		Data: habitTemplateProtoToItem(t),
	}, nil
}
