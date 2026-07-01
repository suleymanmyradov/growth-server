package savedlogic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "SaveItemLogic.SaveItem")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	itemID, err := uuid.Parse(in.ItemId)
	if err != nil {
		l.Errorf("Invalid item ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid item id")
	}

	savedItem, err := l.svcCtx.Repo.SavedItems.CreateSavedItem(ctx, (in.ItemType), itemID, userID)
	if err != nil {
		l.Errorf("Failed to save item: %v", err)
		return nil, status.Error(codes.Internal, "failed to save item")
	}

	// Log article_saved activity when an article is saved
	if in.ItemType == "article" {
		desc := fmt.Sprintf("Saved article: %s", in.ItemId)
		meta, _ := json.Marshal(map[string]string{
			"itemId":   in.ItemId,
			"itemType": in.ItemType,
		})
		if _, err := l.svcCtx.Repo.Activities.CreateActivity(ctx, db.CreateActivityParams{
			Type:        "article_saved",
			Title:       "Saved an article",
			Description: &desc,
			Metadata:    meta,
			UserID:      userID,
		}); err != nil {
			l.Errorf("Failed to log article_saved activity: %v", err)
		}
	}

	return &client.SaveItemResponse{
		SavedId: savedItem.ID.String(),
	}, nil
}
