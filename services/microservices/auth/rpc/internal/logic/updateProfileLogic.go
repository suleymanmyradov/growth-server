package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateProfileLogic.UpdateProfile")
	defer span.End()

	l.Infof("UpdateProfile attempt for user: %s", in.UserId)

	if in == nil || in.UserId == "" {
		l.Errorf("UpdateProfile validation failed: user ID is required")
		return nil, errInvalidArgument(MsgUserIdRequired)
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("UpdateProfile failed to parse user ID: %v", err)
		return nil, errInvalidArgument(MsgInvalidUserId)
	}

	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, userID)
	if err != nil {
		l.Errorf("UpdateProfile failed to get user %s: %v", userID, err)
		return nil, ErrUserNotFound
	}

	if in.FullName != "" {
		user, err = l.svcCtx.Repo.Users.UpdateUserFullName(ctx, user.ID, in.FullName)
		if err != nil {
			l.Errorf("UpdateProfile failed to update user full name for user %s: %v", userID, err)
			return nil, errInternal(MsgFailedUpdateUser)
		}
	}

	user, err = l.svcCtx.Repo.Users.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:        userID,
		Bio:       toNullString(in.Bio),
		Location:  toNullString(in.Location),
		Website:   toNullString(in.Website),
		Interests: in.Interests,
		AvatarUrl: toNullString(in.AvatarUrl),
	})
	if err != nil {
		l.Errorf("UpdateProfile failed to update profile for user %s: %v", userID, err)
		return nil, errInternal(MsgFailedUpdateProfile)
	}

	l.Infof("UpdateProfile successful for user %s", userID)

	// Publish synchronously so broker issues are visible immediately in dev.
	publishUserProfileUpdated(ctx, l.svcCtx.EventsPub, user)

	return &auth.UpdateProfileResponse{
		User: toPbUser(user),
	}, nil
}
