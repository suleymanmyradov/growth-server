package checkinservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetCheckInHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCheckInHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCheckInHistoryLogic {
	return &GetCheckInHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCheckInHistoryLogic) GetCheckInHistory(in *client.GetCheckInHistoryRequest) (*client.GetCheckInHistoryResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetCheckInHistoryLogic.GetCheckInHistory")
	defer span.End()
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (in.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	var checkIns []db.CheckIn
	var habitID uuid.UUID
	if in.HabitId != "" {
		habitID, err = uuid.Parse(in.HabitId)
		if err != nil {
			l.Errorf("Invalid habit ID: %v", err)
			return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
		}
		checkIns, err = l.svcCtx.Repo.CheckIns.GetCheckInsByHabit(ctx, habitID, userID, limit, offset)
	} else {
		checkIns, err = l.svcCtx.Repo.CheckIns.GetCheckInsByUser(ctx, userID, limit, offset)
	}

	if err != nil {
		l.Errorf("Failed to get check-in history: %v", err)
		return nil, status.Error(codes.Internal, "failed to get check-in history")
	}

	pbCheckIns := make([]*client.CheckIn, len(checkIns))
	for i, ci := range checkIns {
		pbCheckIns[i] = checkInToProto(ci)
	}

	// Get total count
	var total int64
	if in.HabitId != "" {
		total, err = l.svcCtx.Repo.CheckIns.CountCheckInsByHabit(ctx, habitID)
		if err != nil {
			l.Errorf("Failed to count check-ins by habit: %v", err)
			total = 0
		}
	} else {
		total, err = l.svcCtx.Repo.CheckIns.CountCheckInsByUser(ctx, userID)
		if err != nil {
			l.Errorf("Failed to count check-ins by user: %v", err)
			total = 0
		}
	}

	return &client.GetCheckInHistoryResponse{
		CheckIns: pbCheckIns,
		Total:    int32(total),
	}, nil
}
