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

type SaveItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveItemLogic {
	return &SaveItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SaveItemLogic) SaveItem(in *client.SaveItemRequest) (*client.SaveItemResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	itemID, err := uuid.Parse(in.ItemId)
	if err != nil {
		l.Errorf("Invalid item ID: %v", err)
		return nil, err
	}

	params := db.CreateSavedItemParams{
		UserID:   userID,
		ItemID:   itemID,
		ItemType: db.SavedItemType(in.ItemType),
	}

	savedItem, err := l.svcCtx.Repo.SavedItems.CreateSavedItem(l.ctx, params)
	if err != nil {
		l.Errorf("Failed to save item: %v", err)
		return nil, err
	}

	return &client.SaveItemResponse{
		SavedId: savedItem.ID.String(),
	}, nil
}
