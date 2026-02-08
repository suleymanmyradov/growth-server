package activitylogic

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogActivityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogActivityLogic {
	return &LogActivityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogActivityLogic) LogActivity(in *client.LogActivityRequest) (*client.LogActivityResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	var metadata pqtype.NullRawMessage
	if len(in.Metadata) > 0 {
		if b, err := json.Marshal(in.Metadata); err == nil {
			metadata = pqtype.NullRawMessage{Valid: true, RawMessage: b}
		}
	}

	activity, err := l.svcCtx.Repo.Activities.LogActivity(l.ctx, db.LogActivityParams{
		UserID:      userID,
		ItemType:    in.Type,
		Title:       in.Description,
		Description: sql.NullString{String: in.Description, Valid: in.Description != ""},
		Metadata:    metadata,
	})
	if err != nil {
		l.Logger.Errorf("Failed to create activity: %v", err)
		return nil, err
	}

	return &client.LogActivityResponse{
		ActivityId: activity.ID.String(),
	}, nil
}
