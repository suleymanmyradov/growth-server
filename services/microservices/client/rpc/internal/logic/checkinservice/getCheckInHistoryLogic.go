package checkinservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	if in.HabitId != "" {
		habitID, err := uuid.Parse(in.HabitId)
		if err != nil {
			l.Errorf("Invalid habit ID: %v", err)
			return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
		}
		checkIns, err = l.svcCtx.Repo.CheckIns.GetCheckInsByHabit(l.ctx, habitID, limit, offset)
	} else {
		checkIns, err = l.svcCtx.Repo.CheckIns.GetCheckInsByUser(l.ctx, userID, limit, offset)
	}

	if err != nil {
		l.Errorf("Failed to get check-in history: %v", err)
		return nil, status.Error(codes.Internal, "failed to get check-in history")
	}

	var pbCheckIns []*client.CheckIn
	for _, ci := range checkIns {
		pbCheckIns = append(pbCheckIns, checkInToProto(ci))
	}

	// Get total count
	var total int64
	if in.HabitId != "" {
		habitID, _ := uuid.Parse(in.HabitId)
		total, err = l.svcCtx.Repo.CheckIns.CountCheckInsByHabit(l.ctx, habitID)
		if err != nil {
			l.Errorf("Failed to count check-ins by habit: %v", err)
			total = 0
		}
	} else {
		total, err = l.svcCtx.Repo.CheckIns.CountCheckInsByUser(l.ctx, userID)
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
