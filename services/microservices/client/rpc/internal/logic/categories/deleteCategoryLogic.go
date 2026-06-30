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

type DeleteCategoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCategoryLogic {
	return &DeleteCategoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteCategoryLogic) DeleteCategory(in *client.DeleteCategoryRequest) (*client.DeleteCategoryResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteCategoryLogic.DeleteCategory")
	defer span.End()
	id, err := uuid.Parse(in.CategoryId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid category id")
	}

	// Articles referencing this category have ON DELETE SET NULL, so deletion is
	// safe, but we report how many articles would be un-categorized for visibility.
	count, _ := l.svcCtx.Repo.Categories.CountArticlesByCategory(ctx, id)

	if err := l.svcCtx.Repo.Categories.DeleteCategory(ctx, id); err != nil {
		l.Errorf("delete category failed: %v", err)
		return nil, status.Error(codes.Internal, "delete category failed")
	}

	l.Infof("deleted category %s; %d articles un-categorized", id, count)
	return &client.DeleteCategoryResponse{Success: true}, nil
}
