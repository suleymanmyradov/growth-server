package goaltemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientgoaltemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goaltemplates"
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
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if _, err := l.svcCtx.GoalTemplatesRpc.AdminDeleteGoalTemplate(l.ctx, &clientgoaltemplates.AdminDeleteGoalTemplateRequest{
		Id: req.Id,
	}); err != nil {
		return nil, status.Error(codes.NotFound, "goal template not found")
	}

	return &types.EmptyResponse{}, nil
}
