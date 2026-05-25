package weeklyreviewservicelogic

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetWeeklyReviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeeklyReviewLogic {
	return &GetWeeklyReviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWeeklyReviewLogic) GetWeeklyReview(in *client.GetWeeklyReviewRequest) (*client.GetWeeklyReviewResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Infof("invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	weekStart, err := time.Parse("2006-01-02", in.WeekStart)
	if err != nil {
		l.Infof("invalid weekStart: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid weekStart")
	}

	review, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(l.ctx, userID, weekStart)
	if err != nil {
		if err == sql.ErrNoRows {
			l.Infof("weekly review not found for user %s and week %s", userID, weekStart.Format("2006-01-02"))
			return nil, status.Error(codes.NotFound, "weekly review not found")
		}
		l.Errorf("failed to get weekly review: %v", err)
		return nil, status.Error(codes.Internal, "failed to get weekly review")
	}

	return &client.GetWeeklyReviewResponse{Review: dbReviewToProto(review)}, nil
}
