package weeklyreview

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientweeklyreview "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWeeklyReviewsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListWeeklyReviewsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeeklyReviewsLogic {
	return &ListWeeklyReviewsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListWeeklyReviewsLogic) ListWeeklyReviews(req *types.PageRequest) (resp *types.WeeklyReviewsResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	rpcResp, err := l.svcCtx.WeeklyReviewRpc.ListWeeklyReviews(l.ctx, &clientweeklyreview.ListWeeklyReviewsRequest{
		UserId: p.UserID,
		Page:   int32(req.Page),
		Limit:  int32(req.Limit),
	})
	if err != nil {
		return nil, err
	}

	reviews := make([]types.WeeklyReview, 0, len(rpcResp.Reviews))
	for _, r := range rpcResp.Reviews {
		reviews = append(reviews, ProtoToWeeklyReview(r))
	}

	totalPages := int(rpcResp.Total) / req.Limit
	if int(rpcResp.Total)%req.Limit > 0 {
		totalPages++
	}

	return &types.WeeklyReviewsResponse{
		Data: reviews,
		Page: types.PageResponse{
			Total:      int64(rpcResp.Total),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: totalPages,
		},
	}, nil
}
