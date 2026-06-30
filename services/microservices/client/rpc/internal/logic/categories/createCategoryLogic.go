package categorieslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateCategoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCategoryLogic {
	return &CreateCategoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateCategoryLogic) CreateCategory(in *client.CreateCategoryRequest) (*client.CreateCategoryResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateCategoryLogic.CreateCategory")
	defer span.End()
	if in.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if in.Slug == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}

	cat, err := l.svcCtx.Repo.Categories.CreateCategory(ctx, in.Name, in.Slug, in.SortOrder)
	if err != nil {
		l.Errorf("create category failed: %v", err)
		return nil, status.Error(codes.Internal, "create category failed")
	}

	return &client.CreateCategoryResponse{Category: convertCategory(cat)}, nil
}
