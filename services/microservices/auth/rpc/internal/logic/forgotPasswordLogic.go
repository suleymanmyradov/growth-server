package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

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

	_, err := l.svcCtx.Repo.Users.GetUserByEmail(l.ctx, in.Email)
	if err != nil {
		l.Errorf("user not found for email: %s", in.Email)
		return &auth.EmptyResponse{}, nil
	}

	token := generateRandomToken(32)
	l.Infof("Password reset token for %s: %s (valid for 1 hour)", in.Email, token)

	return &auth.EmptyResponse{}, nil
}

func generateRandomToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

type PasswordReset struct {
	Token     string
	Email     string
	ExpiresAt time.Time
}

type PasswordResetStore struct {
	resets map[string]PasswordReset
}

var globalPasswordResetStore = &PasswordResetStore{
	resets: make(map[string]PasswordReset),
}

func getPasswordReset(token string) (PasswordReset, bool) {
	reset, exists := globalPasswordResetStore.resets[token]
	if !exists {
		return PasswordReset{}, false
	}

	if time.Now().After(reset.ExpiresAt) {
		delete(globalPasswordResetStore.resets, token)
		return PasswordReset{}, false
	}

	return reset, true
}

func deletePasswordReset(token string) {
	delete(globalPasswordResetStore.resets, token)
}
