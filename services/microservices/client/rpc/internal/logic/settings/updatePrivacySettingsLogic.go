package settingslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePrivacySettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePrivacySettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePrivacySettingsLogic {
	return &UpdatePrivacySettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdatePrivacySettingsLogic) UpdatePrivacySettings(in *client.UpdatePrivacySettingsRequest) (*client.UpdatePrivacySettingsResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	l.Logger.Infof("Updating privacy settings for user %s", userID)

	return &client.UpdatePrivacySettingsResponse{
		Success: true,
	}, nil
}
