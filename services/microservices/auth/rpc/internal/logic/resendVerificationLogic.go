package logic

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResendVerificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResendVerificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResendVerificationLogic {
	return &ResendVerificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResendVerification regenerates a verification token and re-sends the email.
// Rate-limited to one resend per 60s per email. Returns success even if the
// email is unknown/already verified to avoid leaking account existence.
func (l *ResendVerificationLogic) ResendVerification(in *auth.ResendVerificationRequest) (*auth.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ResendVerificationLogic.ResendVerification")
	defer span.End()

	if in == nil || in.Email == "" {
		l.Errorf("ResendVerification validation failed: email is required")
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if !validator.IsValidEmail(in.Email) {
		l.Errorf("ResendVerification validation failed: invalid email format: %s", in.Email)
		return nil, status.Error(codes.InvalidArgument, "invalid email format")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(ctx, in.Email)
	if err != nil {
		// Don't reveal whether the email exists.
		l.Infof("ResendVerification: user not found for email %s (no-op)", in.Email)
		return &auth.EmptyResponse{}, nil
	}
	if user.EmailVerified {
		l.Infof("ResendVerification: email %s already verified (no-op)", in.Email)
		return &auth.EmptyResponse{}, nil
	}

	verificationRepo := repository.NewVerificationRepo(l.svcCtx.RedisClient)
	pending, err := verificationRepo.PendingForEmail(ctx, in.Email)
	if err != nil {
		l.Errorf("ResendVerification throttle check failed: %v", err)
		// Fall through — better to allow a resend than to block on a Redis hiccup.
	}
	if pending {
		l.Infof("ResendVerification: throttled for email %s", in.Email)
		return nil, status.Error(codes.ResourceExhausted, "please wait before requesting another verification email")
	}

	token := generateRandomToken(32)
	if err := verificationRepo.Store(ctx, token, user.ID.String(), user.Email, verificationTokenTTL); err != nil {
		l.Errorf("ResendVerification failed to store token: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate verification token")
	}
	if err := verificationRepo.SetThrottle(ctx, user.Email, 60*time.Second); err != nil {
		l.Errorf("ResendVerification failed to set throttle: %v", err)
	}

	verificationURL := l.svcCtx.Config.Email.FrontendBaseURL + "/verify-email?token=" + token
	if err := l.svcCtx.EmailSender.Send(ctx, email.Email{
		To:      []string{user.Email},
		Subject: "Verify your email",
		HTML:    emailVerificationHTML(user.FullName, verificationURL),
	}); err != nil {
		l.Errorf("ResendVerification failed to send email to %s: %v", user.Email, err)
		return nil, status.Error(codes.Internal, "failed to send verification email")
	}

	l.Infof("ResendVerification successful for user %s", user.ID)
	return &auth.EmptyResponse{}, nil
}
