package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type DeleteUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteUserLogic {
	return &DeleteUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteUserLogic) DeleteUser(in *auth.DeleteUserRequest) (*auth.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteUserLogic.DeleteUser")
	defer span.End()

	if in == nil || in.UserId == "" {
		return nil, errInvalidArgument(MsgUserIdRequired)
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("invalid user ID: %v", err)
		return nil, errInvalidArgument(MsgInvalidUserId)
	}

	// Verify the user exists.
	_, err = l.svcCtx.Repo.Users.GetUserByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Delete the user row. Cross-service cleanup is handled by consumers of
	// the user_deleted event (each service owns its own tables).
	if err := l.svcCtx.Repo.Users.DeleteUser(ctx, userID); err != nil {
		l.Errorf("failed to delete user %s: %v", userID, err)
		return nil, errInternal(MsgFailedDeleteUser)
	}

	l.Infof("DeleteUser successful for user %s", userID)

	// Publish synchronously so broker issues are visible immediately in dev.
	if l.svcCtx.EventsPub != nil {
		env, err := events.NewEnvelope(events.TypeUserDeleted, events.UserDeleted{
			UserID: userID.String(),
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("failed to build user_deleted envelope: %v", err)
		} else if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
			logx.WithContext(ctx).Errorf("failed to publish user_deleted event for user %s: %v", userID, err)
		}
	}

	return &auth.EmptyResponse{}, nil
}
