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

type GetCurrentWeeklyReviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCurrentWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrentWeeklyReviewLogic {
	return &GetCurrentWeeklyReviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCurrentWeeklyReviewLogic) GetCurrentWeeklyReview() (resp *types.WeeklyReviewResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	rpcResp, err := l.svcCtx.WeeklyReviewRpc.GetCurrentWeeklyReview(l.ctx, &clientweeklyreview.GetCurrentWeeklyReviewRequest{
		UserId: p.UserID,
	})
	if err != nil {
		// Handle NotFound gracefully - return a well-formed empty review
		// (non-null collections) instead of an error. The client detects the
		// empty state by the absent id.
		if status.Code(err) == codes.NotFound {
			return &types.WeeklyReviewResponse{Data: emptyWeeklyReview()}, nil
		}
		return nil, err
	}

	return &types.WeeklyReviewResponse{Data: ProtoToWeeklyReview(rpcResp.Review)}, nil
}
