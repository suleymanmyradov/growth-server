// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package categories

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientcategories "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/categories"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCategoriesLogic {
	return &ListCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListCategoriesLogic) ListCategories(req *types.ListCategoriesRequest) (resp *types.CategoriesResponse, err error) {
	rpcResp, err := l.svcCtx.CategoriesRpc.ListCategories(l.ctx, &clientcategories.ListCategoriesRequest{
		EntityType: req.EntityType,
	})
	if err != nil {
		return nil, err
	}

	categories := make([]types.Category, 0, len(rpcResp.Categories))
	for _, c := range rpcResp.Categories {
		categories = append(categories, types.Category{
			Id:         c.Id,
			Name:       c.Name,
			Slug:       c.Slug,
			EntityType: req.EntityType,
			SortOrder:  int(c.SortOrder),
			CreatedAt:  formatTime(c.CreatedAt),
			UpdatedAt:  formatTime(c.UpdatedAt),
		})
	}

	return &types.CategoriesResponse{
		Data: categories,
	}, nil
}

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}
