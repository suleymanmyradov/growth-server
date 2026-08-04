package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ForgotPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewForgotPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgotPasswordLogic {
	return &ForgotPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ForgotPasswordLogic) ForgotPassword(in *auth.ForgotPasswordRequest) (*auth.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ForgotPasswordLogic.ForgotPassword")
	defer span.End()

	if in == nil || in.Email == "" {
		l.Errorf("ForgotPassword validation failed: email is required")
		return nil, errInvalidArgument(MsgEmailIsRequired)
	}

	l.Infof("ForgotPassword attempt for email: %s", in.Email)

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(ctx, in.Email)
	if err != nil {
		l.Errorf("ForgotPassword user not found for email: %s", in.Email)
		return &auth.EmptyResponse{}, nil
	}

	token := generateRandomToken(32)
	resetRepo := repository.NewPasswordResetRepo(l.svcCtx.RedisClient)
	if err := resetRepo.Store(ctx, token, user.Email, time.Hour); err != nil {
		l.Errorf("ForgotPassword failed to store password reset token for user %s: %v", user.ID, err)
		return nil, errInternal(MsgFailedProcessPasswordReset)
	}

	resetURL := l.svcCtx.Config.Email.FrontendBaseURL + "/reset-password?token=" + token
	// Detach from the request context so a cancelled RPC (client disconnect,
	// gateway timeout) doesn't prevent the reset email from being sent — the
	// reset token is already stored in Redis and the user can't recover without
	// the email. Give the email send its own generous timeout.
	emailCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := l.svcCtx.EmailSender.Send(emailCtx, email.Email{
		To:      []string{user.Email},
		Subject: "Reset your password",
		HTML:    passwordResetHTML(user.FullName, resetURL),
	}); err != nil {
		l.Errorf("ForgotPassword failed to send reset email to %s: %v", user.Email, err)
		// Don't surface email failures to avoid leaking account existence / config issues.
	}

	l.Infof("ForgotPassword password reset token generated for user %s", user.ID)

	return &auth.EmptyResponse{}, nil
}

func generateRandomToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
