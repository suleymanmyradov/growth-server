package goaltemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientgoaltemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goaltemplates"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListGoalTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListGoalTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListGoalTemplatesLogic {
	return &AdminListGoalTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListGoalTemplatesLogic) AdminListGoalTemplates() (resp *types.GoalTemplatesResponse, err error) {
	rpcResp, err := l.svcCtx.GoalTemplatesRpc.AdminListGoalTemplates(l.ctx, &clientgoaltemplates.AdminListGoalTemplatesRequest{})
	if err != nil {
		return nil, err
	}

	items := make([]types.GoalTemplateItem, 0, len(rpcResp.Templates))
	for _, t := range rpcResp.Templates {
		items = append(items, goalTemplateProtoToItem(t))
	}

	return &types.GoalTemplatesResponse{
		Data: items,
	}, nil
}
