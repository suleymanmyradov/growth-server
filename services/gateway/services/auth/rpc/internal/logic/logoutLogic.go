package logic

import (
	"context"

	authdomain "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/domain/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogoutLogic) Logout(in *auth.LogoutRequest) (*auth.EmptyResponse, error) {
	authCfg := authdomain.AuthConfig{
		JwtSecretKey: l.svcCtx.Config.Auth.AccessSecret,
	}
	manager := authdomain.NewAuthManager(authCfg)

	_, err := manager.ParseTokenV2(in.AccessToken)
	if err != nil {
		return &auth.EmptyResponse{}, nil
	}

	if err := l.svcCtx.Cache.RemoveFromValidTokens(l.ctx, in.AccessToken); err != nil {
		l.Errorf("failed to remove access token from cache on logout: %v", err)
	}

	return &auth.EmptyResponse{}, nil
}
