package checkinservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HasCheckedInTodayLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHasCheckedInTodayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HasCheckedInTodayLogic {
	return &HasCheckedInTodayLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HasCheckedInTodayLogic) HasCheckedInToday(in *client.HasCheckedInTodayRequest) (*client.HasCheckedInTodayResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
	}

	hasCheckedIn, err := l.svcCtx.Repo.CheckIns.HasCheckedInToday(l.ctx, userID, habitID)
	if err != nil {
		l.Errorf("Failed to check if user has checked in today: %v", err)
		return nil, status.Error(codes.Internal, "failed to check check-in status")
	}

	return &client.HasCheckedInTodayResponse{
		CheckedIn: hasCheckedIn,
	}, nil
}
