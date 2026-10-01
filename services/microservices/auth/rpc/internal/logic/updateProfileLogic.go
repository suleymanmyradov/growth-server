package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
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
	// testTxRunner is only set in tests to inject a no-op transaction runner.
	testTxRunner svc.TxRunnerInterface
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

	if in == nil || in.UserId == "" {
		l.Errorf("UpdateProfile validation failed: user ID is required")
		return nil, errInvalidArgument(MsgUserIdRequired)
	}

	l.Infof("UpdateProfile attempt for user: %s", in.UserId)

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

	// Apply the updates and enqueue the profile-sync event atomically (P1):
	// the outbox row commits with the user row, the relay republishes it.
	err = runInTx(l.svcCtx, l.testTxRunner, ctx, userID.String(), func(repo *repository.Repository) error {
		if in.FullName != "" {
			user, err = repo.Users.UpdateUserFullName(ctx, user.ID, in.FullName)
			if err != nil {
				return err
			}
		}
		user, err = repo.Users.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
			ID:        userID,
			Bio:       toNullString(in.Bio),
			Location:  toNullString(in.Location),
			Website:   toNullString(in.Website),
			Interests: in.Interests,
			AvatarUrl: toNullString(in.AvatarUrl),
		})
		if err != nil {
			return err
		}
		return enqueueUserProfileUpdated(ctx, repo.EventOutbox, user)
	})
	if err != nil {
		l.Errorf("UpdateProfile failed to update profile for user %s: %v", userID, err)
		return nil, errInternal(MsgFailedUpdateProfile)
	}

	l.Infof("UpdateProfile successful for user %s", userID)

	return &auth.UpdateProfileResponse{
		User: toPbUser(user),
	}, nil
}
