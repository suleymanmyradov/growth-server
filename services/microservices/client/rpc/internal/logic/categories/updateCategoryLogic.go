package categorieslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateCategoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCategoryLogic {
	return &UpdateCategoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCategoryLogic) UpdateCategory(in *client.UpdateCategoryRequest) (*client.UpdateCategoryResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateCategoryLogic.UpdateCategory")
	defer span.End()
	id, err := uuid.Parse(in.CategoryId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid category id")
	}
	if in.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if in.Slug == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}

	cat, err := l.svcCtx.Repo.Categories.UpdateCategory(ctx, id, in.Name, in.Slug, in.SortOrder)
	if err != nil {
		l.Errorf("update category failed: %v", err)
		return nil, status.Error(codes.Internal, "update category failed")
	}

	return &client.UpdateCategoryResponse{Category: convertCategory(cat)}, nil
}
