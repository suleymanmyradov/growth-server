package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if in == nil || in.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(l.ctx, in.Email)
	if err != nil {
		l.Errorf("user not found for email: %s", in.Email)
		return &auth.EmptyResponse{}, nil
	}

	token := generateRandomToken(32)
	resetRepo := repository.NewPasswordResetRepo(l.svcCtx.RedisClient)
	if err := resetRepo.Store(l.ctx, token, user.Email, time.Hour); err != nil {
		l.Errorf("failed to store password reset token: %v", err)
		return nil, status.Error(codes.Internal, "failed to process password reset")
	}

	l.Infof("password reset token generated for user %s", user.ID)

	return &auth.EmptyResponse{}, nil
}

func generateRandomToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
