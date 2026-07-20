package goaltemplates

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

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
	rows, err := l.svcCtx.Repo.GoalTemplates.List(l.ctx)
	if err != nil {
		return nil, err
	}

	items := make([]types.GoalTemplateItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, goalTemplateRowToItem(row))
	}

	return &types.GoalTemplatesResponse{
		Data: items,
	}, nil
}
