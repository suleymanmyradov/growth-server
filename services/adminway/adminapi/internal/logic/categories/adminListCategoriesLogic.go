package categories

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientcategories "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/categories"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListCategoriesLogic {
	return &AdminListCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListCategoriesLogic) AdminListCategories(req *types.ListCategoriesRequest) (resp *types.CategoriesResponse, err error) {
	rpcReq := &clientcategories.ListCategoriesRequest{
		EntityType: req.EntityType,
	}

	rpcResp, err := l.svcCtx.CategoriesRpc.ListCategories(l.ctx, rpcReq)
	if err != nil {
		return nil, err
	}

	var cats []types.Category
	for _, c := range rpcResp.Categories {
		cats = append(cats, mapCategory(c))
	}

	return &types.CategoriesResponse{
		Data: cats,
	}, nil
}
