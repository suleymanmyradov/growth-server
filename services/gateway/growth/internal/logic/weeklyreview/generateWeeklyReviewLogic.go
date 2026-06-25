package weeklyreview

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	clientweeklyreview "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateWeeklyReviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGenerateWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateWeeklyReviewLogic {
	return &GenerateWeeklyReviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GenerateWeeklyReviewLogic) GenerateWeeklyReview(req *types.GenerateWeeklyReviewRequest) (resp *types.WeeklyReviewResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	rpcResp, err := l.svcCtx.WeeklyReviewRpc.GenerateWeeklyReview(l.ctx, &clientweeklyreview.GenerateWeeklyReviewRequest{
		UserId:          p.UserID,
		WeekStart:       req.WeekStart,
		ForceRegenerate: req.ForceRegenerate,
	})
	if err != nil {
		return nil, err
	}

	return &types.WeeklyReviewResponse{Data: ProtoToWeeklyReview(rpcResp.Review)}, nil
}
