package settingslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetPrivacySettingsLogic.GetPrivacySettings")
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

	l.Infof("Getting privacy settings for user %s", userID)

	return &client.GetPrivacySettingsResponse{}, nil
}
