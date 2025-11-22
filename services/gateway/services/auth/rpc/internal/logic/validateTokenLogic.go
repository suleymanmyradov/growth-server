package logic

import (
	"context"

	authdomain "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/domain/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateTokenLogic {
	return &ValidateTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Token validation
func (l *ValidateTokenLogic) ValidateToken(in *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	authCfg := authdomain.AuthConfig{
		JwtSecretKey: l.svcCtx.Config.Auth.AccessSecret,
	}
	manager := authdomain.NewAuthManager(authCfg)

	payload, err := manager.ParseTokenV2(in.AccessToken)
	if err != nil {
		return &auth.ValidateTokenResponse{Valid: false}, nil
	}

	isV2 := manager.IsPayloadOfAccessTokenV2(payload)
	if isV2 {
		valid, err := l.svcCtx.Cache.IsValidAccessToken(l.ctx, in.AccessToken)
		if err != nil {
			return &auth.ValidateTokenResponse{Valid: false}, nil
		}
		if !valid {
			return &auth.ValidateTokenResponse{Valid: false}, nil
		}
	}

	return &auth.ValidateTokenResponse{
		Valid:    true,
		UserId:   payload.UserId,
		Username: "",
	}, nil
}
