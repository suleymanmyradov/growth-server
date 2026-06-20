package categorieslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListCategoriesLogic.ListCategories")
	defer span.End()
	categories, err := l.svcCtx.Repo.Categories.ListCategories(ctx)
	if err != nil {
		return nil, err
	}

	pbCategories := make([]*client.Category, len(categories))
	for i, c := range categories {
		pbCategories[i] = convertCategory(c)
	}

	return &client.ListCategoriesResponse{
		Categories: pbCategories,
	}, nil
}
