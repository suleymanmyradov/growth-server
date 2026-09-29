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

	// Atomic rotation: consumes the presented token. Previously the old token
	// was never revoked — a stolen admin refresh token stayed valid for its
	// full TTL. With consume-or-replay semantics, concurrent admin refreshes
	// get the same pair and a replayed rotated token revokes the session.
	sess, err := l.svcCtx.TokenMaker.RefreshSession(ctx, req.RefreshToken)
	if err != nil {
		l.Errorf("refresh failed to rotate refresh token: %v", err)
		return nil, ErrInvalidExpiredRefresh
	}
	claims := sess.Claims

	user, err := l.svcCtx.Repo.InternalUsers.GetByID(ctx, claims.Subject)
	if err != nil {
		l.Errorf("refresh failed to get user by id: %v", err)
		return nil, ErrAdminNotFound
	}

	sessionID := claims.SessionID

	roles := []string{user.Role}
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Email, roles, sessionID)
	if err != nil {
		l.Errorf("refresh failed to create access token: %v", err)
		return nil, ErrFailedGenAccessToken
	}

	refreshToken := sess.RefreshToken

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
