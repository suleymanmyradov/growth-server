package logic

import (
	"context"
	"time"

	authdomain "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/domain/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RefreshTokenLogic) RefreshToken(in *auth.RefreshRequest) (*auth.AuthResponse, error) {
	authCfg := authdomain.AuthConfig{
		JwtSecretKey:      l.svcCtx.Config.Auth.AccessSecret,
		AccessTokenExpiry: time.Duration(l.svcCtx.Config.Auth.AccessExpire) * time.Second,
	}
	manager := authdomain.NewAuthManager(authCfg)

	payload, err := manager.ParseTokenV2(in.RefreshToken)
	if err != nil {
		return nil, err
	}

	isV2 := manager.IsPayloadOfAccessTokenV2(payload)
	if isV2 {
		valid, err := l.svcCtx.Cache.IsValidAccessToken(l.ctx, in.RefreshToken)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, nil
		}
	}

	newAccessToken, err := manager.GenerateAccessTokenV2(payload.UserId, payload.DeviceId)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.Cache.AddToValidTokens(l.ctx, newAccessToken); err != nil {
		l.Errorf("failed to add refreshed token to cache: %v", err)
	}
	if isV2 {
		if err := l.svcCtx.Cache.RemoveFromValidTokens(l.ctx, in.RefreshToken); err != nil {
			l.Errorf("failed to remove old token from cache: %v", err)
		}
	}

	accessExpire := l.svcCtx.Config.Auth.AccessExpire
	return &auth.AuthResponse{
		AccessToken: newAccessToken,
		ExpiresIn:   accessExpire,
	}, nil
}
