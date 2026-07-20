// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package habittemplates

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clienthabittemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habittemplates"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListHabitTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListHabitTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHabitTemplatesLogic {
	return &ListHabitTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListHabitTemplatesLogic) ListHabitTemplates() (resp *types.HabitTemplatesResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.HabitTemplates.ListHabitTemplates(l.ctx, &clienthabittemplates.ListHabitTemplatesRequest{})
	if err != nil {
		return nil, err
	}

	items := make([]types.HabitTemplateItem, 0, len(rpcResp.Templates))
	for _, t := range rpcResp.Templates {
		item := types.HabitTemplateItem{
			Id:          t.Id,
			Name:        t.Name,
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

	return &types.HabitTemplatesResponse{
		Data: items,
	}, nil
}

func formatTemplateTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}
