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

type AdminUpdateCategoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateCategoryLogic {
	return &AdminUpdateCategoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateCategoryLogic) AdminUpdateCategory(req *types.UpdateCategoryRequest) (resp *types.CategoryResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "category id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	rpcResp, err := l.svcCtx.CategoriesRpc.UpdateCategory(l.ctx, &clientcategories.UpdateCategoryRequest{
		CategoryId: req.Id,
		Name:       req.Name,
		Slug:       req.Slug,
		SortOrder:  int32(req.SortOrder),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update category via rpc: %w", err)
	}

	if rpcResp.Category == nil {
		return nil, status.Error(codes.Internal, "category update returned nil")
	}

	return &types.CategoryResponse{
		Data: mapCategory(rpcResp.Category),
	}, nil
}
