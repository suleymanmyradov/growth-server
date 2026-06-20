package auth

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type AuthRefreshLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthRefreshLogic {
	return &AuthRefreshLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AuthRefreshLogic) AuthRefresh(req *types.RefreshTokenRequest) (*types.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "AuthRefreshLogic.AuthRefresh")
	defer span.End()

	claims, err := l.svcCtx.TokenMaker.VerifyRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		l.Errorf("refresh failed to verify refresh token: %v", err)
		return nil, err
	}

	user, err := l.svcCtx.Repo.InternalUsers.GetByID(ctx, claims.Subject)
	if err != nil {
		l.Errorf("refresh failed to get user by id: %v", err)
		return nil, err
	}

	sessionID := claims.SessionID

	roles := []string{user.Role}
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Email, roles, sessionID)
	if err != nil {
		l.Errorf("refresh failed to create access token: %v", err)
		return nil, err
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Email, roles, sessionID)
	if err != nil {
		l.Errorf("refresh failed to create refresh token: %v", err)
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
