package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"golang.org/x/crypto/bcrypt"
)

type AuthLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthLoginLogic {
	return &AuthLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AuthLoginLogic) AuthLogin(req *types.LoginRequest) (*types.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AuthLoginLogic.AuthLogin")
	defer span.End()

	user, err := l.svcCtx.Repo.InternalUsers.GetByEmail(ctx, req.Email)
	if err != nil {
		l.Errorf("login failed to get internal user by email: %v", err)
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		l.Errorf("login password mismatch for user %s: %v", user.ID, err)
		return nil, err
	}

	sessionID := uuid.New()

	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Email, []string{user.Role}, sessionID)
	if err != nil {
		l.Errorf("login failed to create access token for user %s: %v", user.ID, err)
		return nil, err
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Email, []string{user.Role}, sessionID)
	if err != nil {
		l.Errorf("login failed to create refresh token for user %s: %v", user.ID, err)
		return nil, err
	}

	return &types.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		User: types.UserInfo{
			Id:       user.ID.String(),
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	}, nil
}
