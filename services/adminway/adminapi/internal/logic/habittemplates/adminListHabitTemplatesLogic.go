package habittemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

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
	rows, err := l.svcCtx.Repo.HabitTemplates.List(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]types.HabitTemplateItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, habitTemplateRowToItem(row))
	}

	return &types.HabitTemplatesResponse{
		Data: items,
	}, nil
}
