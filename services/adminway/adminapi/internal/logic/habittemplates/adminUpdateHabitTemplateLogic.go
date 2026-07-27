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

type AdminUpdateHabitTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateHabitTemplateLogic {
	return &AdminUpdateHabitTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateHabitTemplateLogic) AdminUpdateHabitTemplate(req *types.UpdateHabitTemplateRequest) (resp *types.HabitTemplateResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	categoryId := ""
	if req.CategoryId != nil {
		categoryId = *req.CategoryId
	}

	t, err := l.svcCtx.HabitTemplatesRpc.AdminUpdateHabitTemplate(l.ctx, &clienthabittemplates.AdminUpdateHabitTemplateRequest{
		Id:          req.Id,
		Name:        req.Name,
		Description: req.Description,
		CategoryId:  categoryId,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "habit template not found")
	}

	return &types.HabitTemplateResponse{
		Data: habitTemplateProtoToItem(t),
	}, nil
}
