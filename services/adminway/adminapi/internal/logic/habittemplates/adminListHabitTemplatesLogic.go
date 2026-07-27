package habittemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clienthabittemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habittemplates"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListHabitTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListHabitTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListHabitTemplatesLogic {
	return &AdminListHabitTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListHabitTemplatesLogic) AdminListHabitTemplates() (resp *types.HabitTemplatesResponse, err error) {
	rpcResp, err := l.svcCtx.HabitTemplatesRpc.AdminListHabitTemplates(l.ctx, &clienthabittemplates.AdminListHabitTemplatesRequest{})
	if err != nil {
		return nil, err
	}

	items := make([]types.HabitTemplateItem, 0, len(rpcResp.Templates))
	for _, t := range rpcResp.Templates {
		items = append(items, habitTemplateProtoToItem(t))
	}

	return &types.HabitTemplatesResponse{
		Data: items,
	}, nil
}
