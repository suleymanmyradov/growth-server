package settingslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPrivacySettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPrivacySettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPrivacySettingsLogic {
	return &GetPrivacySettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPrivacySettingsLogic) GetPrivacySettings(in *client.GetPrivacySettingsRequest) (*client.GetPrivacySettingsResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	l.Logger.Infof("Getting privacy settings for user %s", userID)

	return &client.GetPrivacySettingsResponse{}, nil
}
