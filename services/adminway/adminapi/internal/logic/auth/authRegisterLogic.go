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

type AuthRegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthRegisterLogic {
	return &AuthRegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AuthRegisterLogic) AuthRegister(req *types.RegisterRequest) (*types.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AuthRegisterLogic.AuthRegister")
	defer span.End()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("register failed to hash password: %v", err)
		return nil, err
	}

	role := "admin"
	if req.Role != "" {
		role = req.Role
	}

	user, err := l.svcCtx.Repo.InternalUsers.Create(ctx, req.Email, string(hashedPassword), req.FullName, role)
	if err != nil {
		l.Errorf("register failed to create internal user: %v", err)
		return nil, err
	}

	sessionID := uuid.New()

	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Email, []string{user.Role}, sessionID)
	if err != nil {
		l.Errorf("register failed to create access token for user %s: %v", user.ID, err)
		return nil, err
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Email, []string{user.Role}, sessionID)
	if err != nil {
		l.Errorf("register failed to create refresh token for user %s: %v", user.ID, err)
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
