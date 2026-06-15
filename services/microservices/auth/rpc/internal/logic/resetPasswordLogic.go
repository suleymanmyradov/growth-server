package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResetPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordLogic {
	return &ResetPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetPasswordLogic) ResetPassword(in *auth.ResetPasswordRequest) (*auth.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ResetPasswordLogic.ResetPassword")
	defer span.End()

	l.Infof("ResetPassword attempt")

	if in == nil || in.Token == "" || in.NewPassword == "" {
		l.Errorf("ResetPassword validation failed: token and new password are required")
		return nil, status.Error(codes.InvalidArgument, "token and new password are required")
	}

	resetRepo := repository.NewPasswordResetRepo(l.svcCtx.RedisClient)
	entry, exists, err := resetRepo.Get(ctx, in.Token)
	if err != nil {
		l.Errorf("ResetPassword failed to lookup reset token: %v", err)
		return nil, status.Error(codes.Internal, "failed to validate reset token")
	}
	if !exists {
		l.Errorf("ResetPassword invalid or expired reset token")
		return nil, status.Error(codes.Unauthenticated, "invalid or expired reset token")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(ctx, entry.Email)
	if err != nil {
		l.Errorf("ResetPassword failed to get user for email %s: %v", entry.Email, err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("ResetPassword failed to hash new password for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to process new password")
	}

	_, err = l.svcCtx.Repo.Users.UpdateUserPassword(ctx, user.ID, string(hashedPassword))
	if err != nil {
		l.Errorf("ResetPassword failed to update password for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to update password")
	}

	if err := resetRepo.Delete(ctx, in.Token); err != nil {
		l.Errorf("ResetPassword failed to delete reset token: %v", err)
	}

	l.Infof("ResetPassword successful for user %s", user.ID)

	return &auth.EmptyResponse{}, nil
}
