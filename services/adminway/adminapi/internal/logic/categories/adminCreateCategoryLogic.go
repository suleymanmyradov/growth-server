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

type AdminCreateCategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCreateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateCategoryLogic {
	return &AdminCreateCategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateCategoryLogic) AdminCreateCategory(req *types.CreateCategoryRequest) (resp *types.CategoryResponse, err error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	rpcResp, err := l.svcCtx.CategoriesRpc.CreateCategory(l.ctx, &clientcategories.CreateCategoryRequest{
		Name:      req.Name,
		Slug:      req.Slug,
		SortOrder: int32(req.SortOrder),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create category via rpc: %w", err)
	}

	if rpcResp.Category == nil {
		return nil, status.Error(codes.Internal, "category creation returned nil")
	}

	return &types.CategoryResponse{
		Data: mapCategory(rpcResp.Category),
	}, nil
}
