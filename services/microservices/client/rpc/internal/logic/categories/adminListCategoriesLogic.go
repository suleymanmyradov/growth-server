package categorieslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type AdminListCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminListCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListCategoriesLogic {
	return &AdminListCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminListCategoriesLogic) AdminListCategories(in *client.AdminListCategoriesRequest) (*client.AdminListCategoriesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AdminListCategoriesLogic.AdminListCategories")
	defer span.End()

	categories, err := l.svcCtx.Repo.Categories.AdminListCategories(ctx)
	if err != nil {
		return nil, err
	}

	pbCats := make([]*client.TemplateCategory, len(categories))
	for i, c := range categories {
		pbCats[i] = &client.TemplateCategory{
			Id:   c.ID.String(),
			Name: c.Name,
			Slug: c.Slug,
		}
	}

	return &client.AdminListCategoriesResponse{
		Categories: pbCats,
	}, nil
}
