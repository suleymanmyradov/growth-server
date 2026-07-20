// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package goaltemplates

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoaltemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goaltemplates"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGoalTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListGoalTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGoalTemplatesLogic {
	return &ListGoalTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListGoalTemplatesLogic) ListGoalTemplates() (resp *types.GoalTemplatesResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.GoalTemplates.ListGoalTemplates(l.ctx, &clientgoaltemplates.ListGoalTemplatesRequest{})
	if err != nil {
		return nil, err
	}

	items := make([]types.GoalTemplateItem, 0, len(rpcResp.Templates))
	for _, t := range rpcResp.Templates {
		item := types.GoalTemplateItem{
			Id:          t.Id,
			Title:       t.Title,
			Description: t.Description,
			SortOrder:   t.SortOrder,
			CreatedAt:   formatTemplateTime(t.CreatedAt),
			UpdatedAt:   formatTemplateTime(t.UpdatedAt),
		}
		if t.Category != nil {
			item.Category = &types.TemplateCategory{
				Id:   t.Category.Id,
				Name: t.Category.Name,
				Slug: t.Category.Slug,
			}
		}
		items = append(items, item)
	}

	return &types.GoalTemplatesResponse{
		Data: items,
	}, nil
}

func formatTemplateTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}
