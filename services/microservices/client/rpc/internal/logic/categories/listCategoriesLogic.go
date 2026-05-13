package categorieslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
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
	entityType := db.EntityType(in.EntityType)
	categories, err := l.svcCtx.Repo.Categories.ListCategories(l.ctx, entityType)
	if err != nil {
		return nil, err
	}

	var pbCategories []*client.Category
	for _, c := range categories {
		sortOrder := int32(0)
		if c.SortOrder.Valid {
			sortOrder = c.SortOrder.Int32
		}
		pbCategories = append(pbCategories, &client.Category{
			Id:         c.ID.String(),
			Name:       c.Name,
			Slug:       c.Slug,
			EntityType: string(c.EntityType),
			SortOrder:  sortOrder,
			CreatedAt:  c.CreatedAt.Unix(),
			UpdatedAt:  c.UpdatedAt.Unix(),
		})
	}

	return &client.ListCategoriesResponse{
		Categories: pbCategories,
	}, nil
}
