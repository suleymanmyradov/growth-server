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

type ReorderCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReorderCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReorderCategoriesLogic {
	return &ReorderCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReorderCategoriesLogic) ReorderCategories(in *client.ReorderCategoriesRequest) (*client.ReorderCategoriesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ReorderCategoriesLogic.ReorderCategories")
	defer span.End()
	if len(in.Ids) == 0 || len(in.Ids) != len(in.SortOrders) {
		return nil, status.Error(codes.InvalidArgument, "ids and sortOrders must be non-empty and equal in length")
	}

	ids := make([]uuid.UUID, 0, len(in.Ids))
	for _, raw := range in.Ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid category id")
		}
		ids = append(ids, id)
	}

	if err := l.svcCtx.Repo.Categories.ReorderCategories(ctx, ids, in.SortOrders); err != nil {
		l.Errorf("reorder categories failed: %v", err)
		return nil, status.Error(codes.Internal, "reorder categories failed")
	}

	return &client.ReorderCategoriesResponse{Success: true}, nil
}
