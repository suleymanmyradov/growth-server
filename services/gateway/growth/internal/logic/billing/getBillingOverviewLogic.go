package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBillingOverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetBillingOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBillingOverviewLogic {
	return &GetBillingOverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetBillingOverviewLogic) GetBillingOverview() (resp *types.BillingOverviewResponse, err error) {
	rpcResp, err := l.svcCtx.BillingRpc.GetBillingOverview(l.ctx, &clientbilling.GetBillingOverviewRequest{})
	if err != nil {
		return nil, err
	}

	var plans []types.Plan
	for _, p := range rpcResp.Plans {
		plans = append(plans, types.Plan{
			Id:                       p.Id,
			Code:                     p.Code,
			Name:                     p.Name,
			Description:              p.Description,
			PriceMonthlyCents:        int(p.PriceMonthlyCents),
			PriceAnnualCents:         int(p.PriceAnnualCents),
			ActiveGoalLimit:          int(p.ActiveGoalLimit),
			ActiveHabitLimit:         int(p.ActiveHabitLimit),
			WeeklyReviewHistoryLimit: int(p.WeeklyReviewHistoryLimit),
			PlanAdjustmentLimit:      int(p.PlanAdjustmentLimit),
			PersonalizedAiEnabled:    p.PersonalizedAiEnabled,
			IsActive:                 p.IsActive,
		})
	}

	sub := types.UserSubscription{
		Id:                rpcResp.Subscription.Id,
		UserId:            rpcResp.Subscription.UserId,
		PlanId:            rpcResp.Subscription.PlanId,
		PlanCode:          rpcResp.Subscription.PlanCode,
		PlanName:          rpcResp.Subscription.PlanName,
		Status:            rpcResp.Subscription.Status,
		BillingInterval:   rpcResp.Subscription.BillingInterval,
		CurrentPeriodStart: rpcResp.Subscription.CurrentPeriodStart,
		CurrentPeriodEnd:  rpcResp.Subscription.CurrentPeriodEnd,
		TrialEnd:          rpcResp.Subscription.TrialEnd,
		CancelAtPeriodEnd: rpcResp.Subscription.CancelAtPeriodEnd,
	}

	ent := types.Entitlements{
		PlanCode:                   rpcResp.Entitlements.PlanCode,
		Status:                     rpcResp.Entitlements.Status,
		ActiveGoalLimit:            int(rpcResp.Entitlements.ActiveGoalLimit),
		ActiveHabitLimit:          int(rpcResp.Entitlements.ActiveHabitLimit),
		WeeklyReviewHistoryLimit:   int(rpcResp.Entitlements.WeeklyReviewHistoryLimit),
		PlanAdjustmentLimit:        int(rpcResp.Entitlements.PlanAdjustmentLimit),
		PersonalizedAiEnabled:      rpcResp.Entitlements.PersonalizedAiEnabled,
		CanCreateGoal:              rpcResp.Entitlements.CanCreateGoal,
		CanCreateHabit:             rpcResp.Entitlements.CanCreateHabit,
		CanViewWeeklyReviewHistory: rpcResp.Entitlements.CanViewWeeklyReviewHistory,
		CanUsePersonalizedAi:       rpcResp.Entitlements.CanUsePersonalizedAi,
		CanCreatePlanAdjustment:    rpcResp.Entitlements.CanCreatePlanAdjustment,
		CurrentActiveGoals:         int(rpcResp.Entitlements.CurrentActiveGoals),
		CurrentActiveHabits:        int(rpcResp.Entitlements.CurrentActiveHabits),
		CurrentPendingAdjustments:  int(rpcResp.Entitlements.CurrentPendingAdjustments),
	}

	return &types.BillingOverviewResponse{
		Data: types.BillingOverviewData{
			Plans:        plans,
			Subscription: sub,
			Entitlements: ent,
			BillingMode:  rpcResp.BillingMode,
		},
	}, nil
}
