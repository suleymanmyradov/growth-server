package logic

import (
	"context"
	"errors"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
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

	// Atomic rotation: consumes the presented token (SET NX), replays the
	// successor pair to stragglers inside the grace window, and revokes the
	// session outright when a rotated token is replayed past grace (theft).
	sess, err := l.svcCtx.TokenMaker.RefreshSession(ctx, in.RefreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrRotationInFlight) {
			// Winner hasn't stored the successor yet — tell the client to
			// retry; do NOT fail the session.
			return nil, errInternal(MsgFailedValidateSession)
		}
		if errors.Is(err, jwt.ErrSessionRevoked) {
			l.Infof("RefreshToken rejected: session revoked (logout or token-reuse detection)")
			return nil, errUnauthenticated(MsgSessionRevoked)
		}
		l.Errorf("RefreshToken failed: %v", err)
		return nil, errUnauthenticated(MsgInvalidOrExpiredRefreshToken)
	}

	sessionID := sess.Claims.SessionID
	userID := sess.Claims.Subject
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

	l.Infof("RefreshToken successful for user %s (replayed=%v)", user.ID, sess.Replayed)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: sess.RefreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}
