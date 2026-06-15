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
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ForgotPasswordLogic.ForgotPassword")
	defer span.End()

	l.Infof("ForgotPassword attempt for email: %s", in.Email)

	if in == nil || in.Email == "" {
		l.Errorf("ForgotPassword validation failed: email is required")
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(ctx, in.Email)
	if err != nil {
		l.Errorf("ForgotPassword user not found for email: %s", in.Email)
		return &auth.EmptyResponse{}, nil
	}

	token := generateRandomToken(32)
	resetRepo := repository.NewPasswordResetRepo(l.svcCtx.RedisClient)
	if err := resetRepo.Store(ctx, token, user.Email, time.Hour); err != nil {
		l.Errorf("ForgotPassword failed to store password reset token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to process password reset")
	}

	l.Infof("ForgotPassword password reset token generated for user %s", user.ID)

	return &auth.EmptyResponse{}, nil
}

func generateRandomToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
