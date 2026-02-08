package settingslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePreferencesLogic {
	return &UpdatePreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdatePreferencesLogic) UpdatePreferences(in *client.UpdatePreferencesRequest) (*client.UpdatePreferencesResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	l.Logger.Infof("Updating preferences for user %s", userID)

	return &client.UpdatePreferencesResponse{
		Success: true,
	}, nil
}
