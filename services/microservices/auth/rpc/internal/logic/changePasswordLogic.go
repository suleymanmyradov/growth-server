package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	l.Infof("ChangePassword attempt for user: %s", in.UserId)

	if in == nil || in.UserId == "" || in.OldPassword == "" || in.NewPassword == "" {
		l.Errorf("ChangePassword validation failed: user ID, old password and new password are required")
		return nil, status.Error(codes.InvalidArgument, "user ID, old password and new password are required")
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("ChangePassword failed to parse user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, userID)
	if err != nil {
		l.Errorf("ChangePassword failed to get user %s: %v", userID, err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.OldPassword))
	if err != nil {
		l.Errorf("ChangePassword invalid old password for user %s: %v", userID, err)
		return nil, status.Error(codes.Unauthenticated, "invalid old password")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("ChangePassword failed to hash new password for user %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "failed to process new password")
	}

	_, err = l.svcCtx.Repo.Users.UpdateUserPassword(ctx, user.ID, string(hashedPassword))
	if err != nil {
		l.Errorf("ChangePassword failed to update password for user %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "failed to update password")
	}

	l.Infof("ChangePassword successful for user %s", userID)

	return &auth.EmptyResponse{}, nil
}
