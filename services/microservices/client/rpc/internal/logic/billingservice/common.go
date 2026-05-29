package billingservicelogic

import (
	"database/sql"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// EntitlementsResult aliases the repository type for use in logic.
type EntitlementsResult = repository.EntitlementsResult

func planToProto(p db.Plan) *client.Plan {
	return &client.Plan{
		Id:                       p.ID.String(),
		Code:                     p.Code,
		Name:                     p.Name,
		Description:              nullStringValue(p.Description),
		PriceMonthlyCents:        p.PriceMonthlyCents,
		PriceAnnualCents:         p.PriceAnnualCents,
		ActiveGoalLimit:          nullInt32Value(p.ActiveGoalLimit),
		ActiveHabitLimit:         nullInt32Value(p.ActiveHabitLimit),
		WeeklyReviewHistoryLimit: nullInt32Value(p.WeeklyReviewHistoryLimit),
		PlanAdjustmentLimit:      nullInt32Value(p.PlanAdjustmentLimit),
		PersonalizedAiEnabled:    p.PersonalizedAiEnabled,
		IsActive:                 p.IsActive,
	}
}

func subscriptionToProto(sub db.GetUserSubscriptionRow) *client.UserSubscription {
	return &client.UserSubscription{
		Id:                   sub.ID.String(),
		UserId:               sub.UserID.String(),
		PlanId:               sub.PlanID.String(),
		PlanCode:             sub.PlanCode,
		PlanName:             sub.PlanName,
		Status:               sub.Status,
		BillingInterval:      nullStringValue(sub.BillingInterval),
		CurrentPeriodStart:   nullTimeValue(sub.CurrentPeriodStart),
		CurrentPeriodEnd:     nullTimeValue(sub.CurrentPeriodEnd),
		TrialEnd:             nullTimeValue(sub.TrialEnd),
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd,
		StripeCustomerId:     nullStringValue(sub.StripeCustomerID),
		StripeSubscriptionId: nullStringValue(sub.StripeSubscriptionID),
	}
}

func entitlementsToProto(e *EntitlementsResult) *client.Entitlements {
	return &client.Entitlements{
		PlanCode:                   e.PlanCode,
		Status:                     e.Status,
		ActiveGoalLimit:            nullInt32Value(e.ActiveGoalLimit),
		ActiveHabitLimit:          nullInt32Value(e.ActiveHabitLimit),
		WeeklyReviewHistoryLimit:   nullInt32Value(e.WeeklyReviewHistoryLimit),
		PlanAdjustmentLimit:        nullInt32Value(e.PlanAdjustmentLimit),
		PersonalizedAiEnabled:      e.PersonalizedAiEnabled,
		CanCreateGoal:              e.CanCreateGoal,
		CanCreateHabit:             e.CanCreateHabit,
		CanViewWeeklyReviewHistory: e.CanViewWeeklyReviewHistory,
		CanUsePersonalizedAi:       e.CanUsePersonalizedAi,
		CanCreatePlanAdjustment:    e.CanCreatePlanAdjustment,
		CurrentActiveGoals:         int32(e.CurrentActiveGoals),
		CurrentActiveHabits:        int32(e.CurrentActiveHabits),
		CurrentPendingAdjustments:  int32(e.CurrentPendingAdjustments),
	}
}

func nullStringValue(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func nullInt32Value(v sql.NullInt32) int32 {
	if v.Valid {
		return v.Int32
	}
	return 0
}

func nullTimeValue(v sql.NullTime) string {
	if v.Valid {
		return v.Time.Format(time.RFC3339)
	}
	return ""
}
