package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type VerifyEmailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyEmailLogic {
	return &VerifyEmailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VerifyEmail validates the token, marks the user's email as verified, and
// issues a fresh JWT pair so the user is logged in immediately after verifying.
func (l *VerifyEmailLogic) VerifyEmail(in *auth.VerifyEmailRequest) (*auth.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "VerifyEmailLogic.VerifyEmail")
	defer span.End()

	if in == nil || in.Token == "" {
		l.Errorf("VerifyEmail validation failed: token is required")
		return nil, errInvalidArgument(MsgTokenIsRequired)
	}

	verificationRepo := repository.NewVerificationRepo(l.svcCtx.RedisClient)
	entry, exists, err := verificationRepo.Get(ctx, in.Token)
	if err != nil {
		l.Errorf("VerifyEmail failed to lookup token: %v", err)
		return nil, errInternal(MsgFailedValidateToken)
	}
	if !exists {
		l.Errorf("VerifyEmail invalid or expired token")
		return nil, errUnauthenticated(MsgInvalidOrExpiredVerifToken)
	}

	userID, err := uuid.Parse(entry.UserID)
	if err != nil {
		l.Errorf("VerifyEmail invalid user id in token: %v", err)
		return nil, errInternal(MsgInvalidVerificationToken)
	}

	user, err := l.svcCtx.Repo.Users.SetEmailVerified(ctx, userID)
	if err != nil {
		l.Errorf("VerifyEmail failed to mark user %s verified: %v", userID, err)
		return nil, errInternal(MsgFailedVerifyEmail)
	}

	if err := verificationRepo.Delete(ctx, in.Token); err != nil {
		l.Errorf("VerifyEmail failed to delete used token: %v", err)
	}

	sessionID := uuid.New()
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("VerifyEmail failed to create access token for user %s: %v", user.ID, err)
		return nil, ErrFailedGenAccessToken
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("VerifyEmail failed to create refresh token for user %s: %v", user.ID, err)
		return nil, ErrFailedGenRefreshTok
	}

	l.Infof("VerifyEmail successful for user %s", user.ID)
	publishUserProfileUpdated(context.WithoutCancel(ctx), l.svcCtx.EventsPub, user)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}
