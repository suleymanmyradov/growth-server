package checkinservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetTodayCheckInsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTodayCheckInsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTodayCheckInsLogic {
	return &GetTodayCheckInsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTodayCheckInsLogic) GetTodayCheckIns(in *client.GetTodayCheckInsRequest) (*client.GetTodayCheckInsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetTodayCheckInsLogic.GetTodayCheckIns")
	defer span.End()
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	checkIns, err := l.svcCtx.Repo.CheckIns.GetTodayCheckIns(ctx, userID)
	if err != nil {
		l.Errorf("Failed to get today's check-ins: %v", err)
		return nil, status.Error(codes.Internal, "failed to get check-ins")
	}

	pbCheckIns := make([]*client.CheckIn, len(checkIns))
	for i, ci := range checkIns {
		pbCheckIns[i] = checkInToProto(ci)
	}

	return &client.GetTodayCheckInsResponse{
		CheckIns: pbCheckIns,
	}, nil
}
