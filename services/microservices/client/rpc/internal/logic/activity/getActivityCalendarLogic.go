package activitylogic

import (
	"context"

	"strconv"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	var year, month int32
	if in.Year != "" {
		y, err := strconv.Atoi(in.Year)
		if err != nil {
			l.Logger.Errorf("Invalid year: %v", err)
			return nil, err
		}
		year = int32(y)
	}

	if in.Month != "" {
		m, err := strconv.Atoi(in.Month)
		if err != nil {
			l.Logger.Errorf("Invalid month: %v", err)
			return nil, err
		}
		month = int32(m)
	}

	rows, err := l.svcCtx.Repo.Activities.GetActivityCalendar(l.ctx, userID, year, month)
	if err != nil {
		l.Logger.Errorf("Failed to get activity calendar: %v", err)
		return nil, err
	}

	var days []*client.CalendarDay
	for _, r := range rows {
		days = append(days, &client.CalendarDay{
			Date:  r.Day.Format("2006-01-02"),
			Count: int32(r.ActivityCount),
		})
	}

	return &client.GetActivityCalendarResponse{Days: days}, nil
}
