package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"golang.org/x/crypto/bcrypt"
)

type ChangePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePasswordLogic {
	return &ChangePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ChangePasswordLogic) ChangePassword(in *auth.ChangePasswordRequest) (*auth.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ChangePasswordLogic.ChangePassword")
	defer span.End()

	if in == nil || in.UserId == "" || in.OldPassword == "" || in.NewPassword == "" {
		l.Errorf("ChangePassword validation failed: user ID, old password and new password are required")
		return nil, errInvalidArgument(MsgUserIdOldNewPasswordReq)
	}

	l.Infof("ChangePassword attempt for user: %s", in.UserId)

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("ChangePassword failed to parse user ID: %v", err)
		return nil, errInvalidArgument(MsgInvalidUserId)
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, userID)
	if err != nil {
		l.Errorf("ChangePassword failed to get user %s: %v", userID, err)
		return nil, ErrUserNotFound
	}

	// OAuth-only users have no local password and cannot change one here.
	if user.PasswordHash == nil {
		l.Errorf("ChangePassword rejected: user %s has no password (OAuth-only account)", userID)
		return nil, errFailedPrecondition(MsgNoPasswordSetForAccount)
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(in.OldPassword))
	if err != nil {
		l.Errorf("ChangePassword invalid old password for user %s: %v", userID, err)
		return nil, errUnauthenticated(MsgInvalidOldPassword)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("ChangePassword failed to hash new password for user %s: %v", userID, err)
		return nil, errInternal(MsgFailedProcessNewPassword)
	}

	_, err = l.svcCtx.Repo.Users.UpdateUserPassword(ctx, user.ID, string(hashedPassword))
	if err != nil {
		l.Errorf("ChangePassword failed to update password for user %s: %v", userID, err)
		return nil, errInternal(MsgFailedUpdatePassword)
	}

	// Kill every session — including this one. A password change is a
	// credential-security event: any stolen token must die with the old
	// password. The client re-logs-in with the new password.
	if err := l.svcCtx.TokenMaker.RevokeAllUserSessions(ctx, userID); err != nil {
		// Non-fatal: the password is already changed. Log loudly; the
		// revocation marker only exists while Redis is up anyway.
		l.Errorf("ChangePassword failed to revoke sessions for user %s: %v", userID, err)
	}

	l.Infof("ChangePassword successful for user %s", userID)

	return &auth.EmptyResponse{}, nil
}
