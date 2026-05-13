package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
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
	if in == nil || in.Token == "" || in.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "token and new password are required")
	}

	reset, exists := getPasswordReset(in.Token)
	if !exists {
		l.Errorf("invalid or expired reset token")
		return nil, status.Error(codes.Unauthenticated, "invalid or expired reset token")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(l.ctx, reset.Email)
	if err != nil {
		l.Errorf("failed to get user: %v", err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("failed to hash new password: %v", err)
		return nil, status.Error(codes.Internal, "failed to process new password")
	}

	_, err = l.svcCtx.Repo.Users.UpdateUserPassword(l.ctx, db.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: string(hashedPassword),
	})
	if err != nil {
		l.Errorf("failed to update password: %v", err)
		return nil, status.Error(codes.Internal, "failed to update password")
	}

	deletePasswordReset(in.Token)

	return &auth.EmptyResponse{}, nil
}
