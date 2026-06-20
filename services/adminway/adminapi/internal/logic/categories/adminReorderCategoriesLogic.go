package categories

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientcategories "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/categories"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminReorderCategoriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReorderCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReorderCategoriesLogic {
	return &AdminReorderCategoriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminReorderCategoriesLogic) AdminReorderCategories(req *types.ReorderCategoriesRequest) (resp *types.EmptyResponse, err error) {
	if len(req.Ids) == 0 || len(req.Ids) != len(req.SortOrders) {
		return nil, status.Error(codes.InvalidArgument, "ids and sortOrders must be non-empty and equal in length")
	}

	sortOrders := make([]int32, len(req.SortOrders))
	for i, v := range req.SortOrders {
		sortOrders[i] = int32(v)
	}

	if _, err := l.svcCtx.CategoriesRpc.ReorderCategories(l.ctx, &clientcategories.ReorderCategoriesRequest{
		Ids:        req.Ids,
		SortOrders: sortOrders,
	}); err != nil {
		return nil, fmt.Errorf("failed to reorder categories via rpc: %w", err)
	}

	return &types.EmptyResponse{}, nil
}
