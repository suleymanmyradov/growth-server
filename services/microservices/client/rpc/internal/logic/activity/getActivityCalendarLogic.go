package activitylogic

import (
	"context"

	"strconv"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetActivityCalendarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetActivityCalendarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityCalendarLogic {
	return &GetActivityCalendarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetActivityCalendarLogic) GetActivityCalendar(in *client.GetActivityCalendarRequest) (*client.GetActivityCalendarResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	var year, month int32
	if in.Year != "" {
		y, err := strconv.Atoi(in.Year)
		if err != nil {
			l.Errorf("Invalid year: %v", err)
return nil, status.Error(codes.Internal, "invalid year")
		}
		year = int32(y)
	}

	if in.Month != "" {
		m, err := strconv.Atoi(in.Month)
		if err != nil {
			l.Errorf("Invalid month: %v", err)
return nil, status.Error(codes.Internal, "invalid month")
		}
		month = int32(m)
	}

	rows, err := l.svcCtx.Repo.Activities.GetActivityCalendar(l.ctx, userID, year, month)
	if err != nil {
		l.Errorf("Failed to get activity calendar: %v", err)
return nil, status.Error(codes.Internal, "failed to get activity calendar")
	}

	days := make([]*client.CalendarDay, len(rows))
	for i, r := range rows {
		days[i] = &client.CalendarDay{
			Date:  r.Day.Time.Format("2006-01-02"),
			Count: int32(r.ActivityCount),
		}
	}

	return &client.GetActivityCalendarResponse{Days: days}, nil
}
