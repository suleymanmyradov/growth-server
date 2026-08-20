package savedlogic

import (
	"context"
	"encoding/json"

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

	// Log activity for saved items (article_saved, goal_saved, habit_saved).
	activityType := in.ItemType + "_saved"
	activityTitle := "Saved " + in.ItemType
	if _, aErr := l.svcCtx.Repo.Activities.CreateActivity(ctx, db.CreateActivityParams{
		Type:        activityType,
		Title:       activityTitle,
		Description: &activityTitle,
		Metadata:    json.RawMessage("{}"),
		UserID:      userID,
	}); aErr != nil {
		l.Errorf("Failed to log %s activity: %v", activityType, aErr)
	}

	return &client.SaveItemResponse{
		SavedId: savedItem.ID.String(),
	}, nil
}
