package activitylogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetStreaksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStreaksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStreaksLogic {
	return &GetStreaksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetStreaksLogic) GetStreaks(in *client.GetStreaksRequest) (*client.GetStreaksResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetStreaksLogic.GetStreaks")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	streaks, err := l.svcCtx.Repo.Activities.GetStreaks(ctx, userID)
	if err != nil {
		l.Errorf("Failed to get streaks: %v", err)
		return nil, status.Error(codes.Internal, "failed to get streaks")
	}

	currentVal, ok := streaks.CurrentStreak.(int64)
	if !ok {
		l.Errorf("Invalid CurrentStreak type")
		return nil, status.Error(codes.Internal, "invalid streak data type")
	}
	longestVal, ok := streaks.LongestStreak.(int64)
	if !ok {
		l.Errorf("Invalid LongestStreak type")
		return nil, status.Error(codes.Internal, "invalid streak data type")
	}
	current := ToInt32(currentVal)
	longest := ToInt32(longestVal)

	return &client.GetStreaksResponse{
		CurrentStreak: current,
		LongestStreak: longest,
	}, nil
}
