package savedlogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListSavedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSavedLogic {
	return &ListSavedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSavedLogic) ListSaved(in *client.ListSavedRequest) (*client.ListSavedResponse, error) {
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	var items []*client.SavedItem
	var totalCount int64

	if in.ItemType != "" {
		dbItems, err := l.svcCtx.Repo.SavedItems.ListSavedItemsByType(l.ctx, userID, in.ItemType, limit, offset)
		if err != nil {
			l.Errorf("Failed to list saved items by type: %v", err)
return nil, status.Error(codes.Internal, "failed to list saved items by type")
		}
		for _, item := range dbItems {
			items = append(items, convertDbSavedItemToPb(item))
		}
	} else {
		dbItems, err := l.svcCtx.Repo.SavedItems.ListSavedItemsByUser(l.ctx, userID, limit, offset)
		if err != nil {
			l.Errorf("Failed to list saved items: %v", err)
return nil, status.Error(codes.Internal, "failed to list saved items")
		}
		for _, item := range dbItems {
			items = append(items, convertDbSavedItemToPb(item))
		}
	}

	totalCount, _ = l.svcCtx.Repo.SavedItems.CountSavedItemsByUser(l.ctx, userID)

	return &client.ListSavedResponse{
		Items:      items,
		TotalCount: int32(totalCount),
	}, nil
}

func convertDbSavedItemToPb(item db.SavedItem) *client.SavedItem {
	return &client.SavedItem{
		Id:       item.ID.String(),
		UserId:   item.UserID.String(),
		ItemId:   item.ItemID.String(),
		ItemType: string(item.ItemType),
		SavedAt:  item.CreatedAt.Time.Unix(),
	}
}
