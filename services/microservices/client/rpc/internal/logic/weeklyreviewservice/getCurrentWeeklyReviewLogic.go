package weeklyreviewservicelogic

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetCurrentWeeklyReviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrentWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrentWeeklyReviewLogic {
	return &GetCurrentWeeklyReviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCurrentWeeklyReviewLogic) GetCurrentWeeklyReview(in *client.GetCurrentWeeklyReviewRequest) (*client.GetCurrentWeeklyReviewResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetCurrentWeeklyReviewLogic.GetCurrentWeeklyReview")
	defer span.End()

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get user timezone
	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, userID)
	if err != nil {
		l.Infof("failed to get user settings, using UTC: %v", err)
	}

	loc := time.UTC
	if settings.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(settings.Timezone)
		if err != nil {
			l.Infof("invalid timezone %s, using UTC: %v", settings.Timezone, err)
			loc = time.UTC
		}
	}

	// Resolve current week start using user's timezone
	weekStart, _, err := resolveWeekBounds("", loc)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to resolve week bounds")
	}

	// Get the review for the current week
	review, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "weekly review not found")
		}
		l.Errorf("failed to get current weekly review: %v", err)
		return nil, status.Error(codes.Internal, "failed to get current weekly review")
	}

	return &client.GetCurrentWeeklyReviewResponse{Review: dbReviewToProto(review)}, nil
}
