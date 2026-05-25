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

type GetWeeklyReviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeeklyReviewLogic {
	return &GetWeeklyReviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetWeeklyReviewLogic) GetWeeklyReview(weekStart string) (resp *types.WeeklyReviewResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	rpcResp, err := l.svcCtx.WeeklyReviewRpc.GetWeeklyReview(l.ctx, &clientweeklyreview.GetWeeklyReviewRequest{
		UserId:    p.UserID,
		WeekStart: weekStart,
	})
	if err != nil {
		return nil, err
	}

	return &types.WeeklyReviewResponse{Data: protoToWeeklyReview(rpcResp.Review)}, nil
}
