package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProfileLogic {
	return &UpdateProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProfileLogic) UpdateProfile(in *auth.UpdateProfileRequest) (*auth.UpdateProfileResponse, error) {
	if in == nil || in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user ID is required")
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("failed to parse user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(l.ctx, userID)
	if err != nil {
		l.Errorf("failed to get user: %v", err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	if in.FullName != "" {
		user, err = l.svcCtx.Repo.Users.UpdateUserFullName(l.ctx, user.ID, in.FullName)
		if err != nil {
			l.Errorf("failed to update user full name: %v", err)
			return nil, status.Error(codes.Internal, "failed to update user")
		}
	}

	profile, err := l.svcCtx.Repo.Profiles.UpdateProfile(l.ctx, db.UpdateProfileParams{
		UserID:    userID,
		Bio:       toNullString(in.Bio),
		Location:  toNullString(in.Location),
		Website:   toNullString(in.Website),
		Interests: in.Interests,
		AvatarUrl: toNullString(in.AvatarUrl),
	})
	if err != nil {
		l.Errorf("failed to update profile: %v", err)
		return nil, status.Error(codes.Internal, "failed to update profile")
	}

	return &auth.UpdateProfileResponse{
		User: toPbUser(user, profile),
	}, nil
}
