// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthLogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthLogoutLogic {
	return &AuthLogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuthLogout revokes the caller's session so every refresh token issued for it
// (including copies) is rejected from now on. Revoking the access token itself
// is best-effort — it expires quickly anyway.
func (l *AuthLogoutLogic) AuthLogout() (resp *types.EmptyResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok || p.SessionID == "" {
		l.Errorf("logout failed: no authenticated principal in context")
		return nil, ErrInvalidExpiredRefresh
	}

	sessionID, err := uuid.Parse(p.SessionID)
	if err != nil {
		l.Errorf("logout failed to parse session id: %v", err)
		return nil, ErrInvalidExpiredRefresh
	}

	// Revoke the entire session: every refresh token issued for it is rejected
	// on the next refresh attempt. This is the critical operation — if it
	// fails the session is still active, so surface the error to the client.
	sessionTTL := l.svcCtx.Config.Auth.RefreshExpiryDuration
	if err := l.svcCtx.TokenMaker.RevokeSession(l.ctx, sessionID, sessionTTL); err != nil {
		l.Errorf("logout failed to revoke session %s: %v", p.SessionID, err)
		return nil, errInternal(MsgFailedRevokeSession)
	}

	// Best-effort: revoke the presented access token by value. It expires in
	// minutes anyway; failure here does not compromise the session revocation.
	if token, ok := principal.TokenFrom(l.ctx); ok {
		if err := l.svcCtx.TokenMaker.RevokeAccessToken(l.ctx, token); err != nil {
			l.Errorf("logout failed to revoke access token: %v", err)
		}
	}

	l.Infof("admin session %s logged out", p.SessionID)
	return &types.EmptyResponse{}, nil
}
