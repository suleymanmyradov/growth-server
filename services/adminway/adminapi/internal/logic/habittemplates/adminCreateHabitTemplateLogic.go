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

type AdminCreateHabitTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCreateHabitTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateHabitTemplateLogic {
	return &AdminCreateHabitTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateHabitTemplateLogic) AdminCreateHabitTemplate(req *types.CreateHabitTemplateRequest) (resp *types.HabitTemplateResponse, err error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	categoryId := ""
	if req.CategoryId != nil {
		categoryId = *req.CategoryId
	}

	t, err := l.svcCtx.HabitTemplatesRpc.AdminCreateHabitTemplate(l.ctx, &clienthabittemplates.AdminCreateHabitTemplateRequest{
		Name:        req.Name,
		Description: req.Description,
		CategoryId:  categoryId,
		SortOrder:   req.SortOrder,
		IsActive:    req.IsActive,
	})
	if err != nil {
		return nil, err
	}

	return &types.HabitTemplateResponse{
		Data: habitTemplateProtoToItem(t),
	}, nil
}
