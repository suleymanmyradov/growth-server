package settingslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	l.Infof("Updating privacy settings for user %s", userID)

	return &client.UpdatePrivacySettingsResponse{
		Success: true,
	}, nil
}
