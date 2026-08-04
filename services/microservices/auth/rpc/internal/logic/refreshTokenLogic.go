package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "RefreshTokenLogic.RefreshToken")
	defer span.End()

	l.Infof("RefreshToken attempt")

	if in == nil || in.RefreshToken == "" {
		l.Errorf("RefreshToken validation failed: refresh token is required")
		return nil, errInvalidArgument(MsgRefreshTokenRequired)
	}

	refreshClaims, err := l.svcCtx.TokenMaker.VerifyRefreshToken(ctx, in.RefreshToken)
	if err != nil {
		l.Errorf("RefreshToken failed to verify refresh token: %v", err)
		return nil, errUnauthenticated(MsgInvalidOrExpiredRefreshToken)
	}

	// Check whether the session has been revoked (e.g. via logout). This blocks
	// refresh even if the token value itself hasn't been revoked, which is the
	// key protection against a copied refresh token surviving logout.
	sessionID := refreshClaims.SessionID
	revoked, err := l.svcCtx.TokenMaker.IsSessionRevoked(ctx, sessionID)
	if err != nil {
		l.Errorf("RefreshToken failed to check session revocation for %s: %v", sessionID, err)
		return nil, errInternal(MsgFailedValidateSession)
	}
	if revoked {
		l.Infof("RefreshToken rejected: session %s is revoked", sessionID)
		return nil, errUnauthenticated(MsgSessionRevoked)
	}

	userID := refreshClaims.Subject
	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, userID)
	if err != nil {
		l.Errorf("RefreshToken failed to get user %s: %v", userID, err)
		return nil, ErrUserNotFound
	}

	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("RefreshToken failed to create access token for user %s: %v", user.ID, err)
		return nil, ErrFailedGenAccessToken
	}

	newRefreshToken, err := l.svcCtx.TokenMaker.RotateRefreshToken(ctx, in.RefreshToken)
	if err != nil {
		l.Errorf("RefreshToken failed to rotate refresh token for user %s: %v", user.ID, err)
		return nil, ErrFailedGenRefreshTok
	}

	l.Infof("RefreshToken successful for user %s", user.ID)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: newRefreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}
