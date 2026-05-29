package weeklyreviewservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListWeeklyReviewsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWeeklyReviewsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeeklyReviewsLogic {
	return &ListWeeklyReviewsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWeeklyReviewsLogic) ListWeeklyReviews(in *client.ListWeeklyReviewsRequest) (*client.ListWeeklyReviewsResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	page := in.Page
	if page <= 0 {
		page = 1
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	// Enforce weekly review history limit server-side
	var historyLimit int32
	var isRestricted bool
	sub, subErr := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(l.ctx, userID)
	if subErr == nil {
		entitlements, computeErr := l.svcCtx.Repo.Billing.ComputeEntitlements(l.ctx, sub, userID)
		if computeErr == nil && !entitlements.CanViewWeeklyReviewHistory {
			isRestricted = true
			if sub.WeeklyReviewHistoryLimit.Valid && sub.WeeklyReviewHistoryLimit.Int32 > 0 {
				historyLimit = sub.WeeklyReviewHistoryLimit.Int32
			} else {
				historyLimit = 1
			}
			if limit > historyLimit {
				limit = historyLimit
			}
		}
	}

	offset := (page - 1) * limit
	// For restricted users, always start at offset 0 so pagination cannot leak
	// older reviews past the allowed history window.
	if isRestricted {
		offset = 0
	}

	reviews, err := l.svcCtx.Repo.WeeklyReviews.ListWeeklyReviews(l.ctx, userID, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list weekly reviews")
	}

	total, err := l.svcCtx.Repo.WeeklyReviews.CountWeeklyReviews(l.ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to count weekly reviews")
	}

	// Cap total for restricted users so the true count is not leaked.
	if isRestricted && int32(total) > historyLimit {
		total = int64(historyLimit)
	}

	var protoReviews []*client.WeeklyReview
	for _, r := range reviews {
		protoReviews = append(protoReviews, dbReviewToProto(r))
	}

	return &client.ListWeeklyReviewsResponse{
		Reviews: protoReviews,
		Total:   int32(total),
	}, nil
}
