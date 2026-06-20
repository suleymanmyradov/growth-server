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

type AdminDeleteCategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDeleteCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteCategoryLogic {
	return &AdminDeleteCategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteCategoryLogic) AdminDeleteCategory(req *types.ArticleRequest) (resp *types.EmptyResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "category id is required")
	}

	if _, err := l.svcCtx.CategoriesRpc.DeleteCategory(l.ctx, &clientcategories.DeleteCategoryRequest{
		CategoryId: req.Id,
	}); err != nil {
		return nil, fmt.Errorf("failed to delete category via rpc: %w", err)
	}

	return &types.EmptyResponse{}, nil
}
