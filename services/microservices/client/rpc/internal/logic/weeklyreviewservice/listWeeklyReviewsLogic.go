package weeklyreviewservicelogic

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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListWeeklyReviewsLogic.ListWeeklyReviews")
	defer span.End()

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

	// Enforce weekly review history limit server-side.
	// EntitlementsOrFreeFallback applies Free-plan limits when the
	// subscription row can't be loaded — a billing failure degrades a user to
	// Free access, it never skips the check. An error means even the fallback
	// failed; to stay safe we clamp to the strictest window (current week
	// only) instead of returning an unbounded list.
	var historyLimit int32
	var isRestricted bool
	entitlements, entErr := l.svcCtx.Repo.Billing.EntitlementsOrFreeFallback(ctx, userID)
	switch {
	case entErr != nil:
		l.Errorf("ListWeeklyReviews: entitlement check failed closed for user %s, clamping to strictest limit: %v", userID, entErr)
		isRestricted = true
		historyLimit = 1
	case !entitlements.CanViewWeeklyReviewHistory:
		isRestricted = true
		if entitlements.WeeklyReviewHistoryLimit > 0 {
			historyLimit = entitlements.WeeklyReviewHistoryLimit
		} else {
			historyLimit = 1
		}
	}
	if isRestricted && limit > historyLimit {
		limit = historyLimit
	}

	offset := (page - 1) * limit
	// For restricted users, always start at offset 0 so pagination cannot leak
	// older reviews past the allowed history window.
	if isRestricted {
		offset = 0
	}

	reviews, err := l.svcCtx.Repo.WeeklyReviews.ListWeeklyReviews(ctx, userID, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list weekly reviews")
	}

	total, err := l.svcCtx.Repo.WeeklyReviews.CountWeeklyReviews(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to count weekly reviews")
	}

	// Cap total for restricted users so the true count is not leaked.
	if isRestricted && int32(total) > historyLimit {
		total = int64(historyLimit)
	}

	protoReviews := make([]*client.WeeklyReview, len(reviews))
	for i, r := range reviews {
		protoReviews[i] = dbReviewToProto(r)
	}

	return &client.ListWeeklyReviewsResponse{
		Reviews: protoReviews,
		Total:   int32(total),
	}, nil
}
