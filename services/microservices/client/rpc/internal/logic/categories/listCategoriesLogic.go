package categorieslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCategoriesLogic {
	return &ListCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListCategoriesLogic) ListCategories(in *client.ListCategoriesRequest) (*client.ListCategoriesResponse, error) {
	categories, err := l.svcCtx.Repo.Categories.ListCategories(l.ctx)
	if err != nil {
		return nil, err
	}

	pbCategories := make([]*client.Category, len(categories))
	for i, c := range categories {
		pbCategories[i] = &client.Category{
			Id:         c.ID.String(),
			Name:       c.Name,
			Slug:       c.Slug,
			SortOrder:  c.SortOrder,
			CreatedAt:  c.CreatedAt.Time.Unix(),
			UpdatedAt:  c.UpdatedAt.Time.Unix(),
		}
	}

	return &client.ListCategoriesResponse{
		Categories: pbCategories,
	}, nil
}
