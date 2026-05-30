package billingservicelogic

import (
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// EntitlementsResult aliases the repository type for use in logic.
type EntitlementsResult = repository.EntitlementsResult

func planToProto(p db.Plan) *client.Plan {
	description := ""
	if p.Description != nil {
		description = *p.Description
	}
	return &client.Plan{
		Id:                       p.ID.String(),
		Code:                     p.Code,
		Name:                     p.Name,
		Description:              description,
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
	billingInterval := ""
	if sub.BillingInterval != nil {
		billingInterval = string(*sub.BillingInterval)
	}
	return &client.UserSubscription{
		Id:                   sub.ID.String(),
		UserId:               sub.UserID.String(),
		PlanId:               sub.PlanID.String(),
		PlanCode:             sub.PlanCode,
		PlanName:             sub.PlanName,
		Status:               string(sub.Status),
		BillingInterval:      billingInterval,
		CurrentPeriodStart:   pgtypeTimestamptzToString(sub.CurrentPeriodStart),
		CurrentPeriodEnd:     pgtypeTimestamptzToString(sub.CurrentPeriodEnd),
		TrialEnd:             pgtypeTimestamptzToString(sub.TrialEnd),
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd,
		StripeCustomerId:     stringPtrToString(sub.StripeCustomerID),
		StripeSubscriptionId: stringPtrToString(sub.StripeSubscriptionID),
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

func nullInt32Value(v int32) int32 {
	return v
}

func stringPtrToString(v *string) string {
	if v != nil {
		return *v
	}
	return ""
}

func pgtypeTimestamptzToString(v pgtype.Timestamptz) string {
	if v.Valid {
		return v.Time.Format(time.RFC3339)
	}
	return ""
}
