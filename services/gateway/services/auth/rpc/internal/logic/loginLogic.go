package logic

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	authdomain "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/domain/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/shared/models"
	"github.com/suleymanmyradov/growth-server/shared/repository"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *auth.LoginRequest) (*auth.AuthResponse, error) {
	// 1. Find user by email
	var user models.User
	err := l.svcCtx.Repo.GetByID(l.ctx, &user, repository.SelectUserByEmailQuery, in.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// 2. Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password))
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// 3. Generate v2 access token with device awareness
	authCfg := authdomain.AuthConfig{
		JwtSecretKey:      l.svcCtx.Config.Auth.AccessSecret,
		AccessTokenExpiry: time.Duration(l.svcCtx.Config.Auth.AccessExpire) * time.Second,
	}
	manager := authdomain.NewAuthManager(authCfg)

	accessToken, err := manager.GenerateAccessTokenV2(user.ID.String(), in.DeviceId)
	if err != nil {
		return nil, err
	}

	// 4. Store token in Redis as a valid access token
	if err := l.svcCtx.Cache.AddToValidTokens(l.ctx, accessToken); err != nil {
		l.Errorf("failed to add access token to cache: %v", err)
		// continue returning token; depending on requirements, this could be a hard error
	}

	accessExpire := l.svcCtx.Config.Auth.AccessExpire
	return &auth.AuthResponse{
		AccessToken: accessToken,
		ExpiresIn:   accessExpire,
	}, nil
}
