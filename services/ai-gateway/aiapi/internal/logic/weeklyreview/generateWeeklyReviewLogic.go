package weeklyreview

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	clientweeklyreview "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// Step 1: Prepare — compute stats, check cache/cooldown.
	prepResp, err := l.svcCtx.ClientRpc.WeeklyReviewService.PrepareWeeklyReview(l.ctx, &clientweeklyreview.PrepareWeeklyReviewRequest{
		UserId:          p.UserID,
		WeekStart:       req.WeekStart,
		ForceRegenerate: req.ForceRegenerate,
	})
	if err != nil {
		return nil, err
	}

	// If a cached review exists, return it directly.
	if prepResp.ExistingReview != nil {
		return &types.WeeklyReviewResponse{Data: ProtoToWeeklyReview(prepResp.ExistingReview)}, nil
	}

	if prepResp.Data == nil {
		return nil, status.Error(codes.Internal, "prepare returned no data and no existing review")
	}

	// Step 2: Call the ai-coach RPC directly (gateway orchestrates).
	aiReq := BuildWeeklyReviewAIRequest(prepResp.Data)
	aiResp, aiErr := l.svcCtx.AICoachRpc.AICoachService.GenerateWeeklyReview(l.ctx, aiReq, zrpc.WithCallTimeout(90*time.Second))

	var aiSummary string
	var suggestedAdjustments []*aicoachservice.WeeklyReviewAdjustment
	var nextWeekPlan *aicoachservice.NextWeekPlan
	if aiErr != nil {
		l.Errorf("AI generation failed, using fallback: %v", aiErr)
		aiSummary = "I couldn't generate a full weekly review right now. Based on your recent activity, focus on maintaining consistency with your core habits and address any recurring blockers you noticed this week."
		nextWeekPlan = &aicoachservice.NextWeekPlan{
			Focus:           "Maintain consistency",
			Commitments:     []string{"Complete at least one habit check-in each day"},
			Risks:           []string{"Falling behind on new habits"},
			RecoveryActions: []string{"Scale back to one core habit if overwhelmed"},
		}
	} else {
		aiSummary = aiResp.AiSummary
		suggestedAdjustments = aiResp.SuggestedAdjustments
		nextWeekPlan = aiResp.NextWeekPlan
	}

	// Step 3: Save — persist to DB.
	saveResp, err := l.svcCtx.ClientRpc.WeeklyReviewService.SaveWeeklyReview(l.ctx, &clientweeklyreview.SaveWeeklyReviewRequest{
		Data:                 prepResp.Data,
		AiSummary:            aiSummary,
		SuggestedAdjustments: ConvertAdjustmentsToClient(suggestedAdjustments),
		NextWeekPlan:         ConvertNextWeekPlanToClient(nextWeekPlan),
	})
	if err != nil {
		return nil, err
	}

	return &types.WeeklyReviewResponse{Data: ProtoToWeeklyReview(saveResp.Review)}, nil
}
