package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type billingRepo struct {
	db *db.Queries
}

func NewBillingRepo(queries *db.Queries) IBilling {
	return &billingRepo{db: queries}
}

func (r *billingRepo) ListActivePlans(ctx context.Context) ([]db.Plan, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.ListActivePlans")
	defer span.End()

	return r.db.ListActivePlans(ctx)
}

func (r *billingRepo) GetPlanByCode(ctx context.Context, code string) (db.Plan, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetPlanByCode")
	defer span.End()

	return r.db.GetPlanByCode(ctx, code)
}

func (r *billingRepo) GetUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetUserSubscription")
	defer span.End()

	return r.db.GetUserSubscription(ctx, userID)
}

func (r *billingRepo) GetOrCreateUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetOrCreateUserSubscription")
	defer span.End()

	sub, err := r.db.GetUserSubscription(ctx, userID)
	if err != nil {
		// If no subscription exists, create a free one automatically
		_, createErr := r.db.CreateDefaultFreeSubscription(ctx, userID)
		if createErr != nil {
			return db.GetUserSubscriptionRow{}, createErr
		}
		// Fetch the newly created subscription
		sub, err = r.db.GetUserSubscription(ctx, userID)
		if err != nil {
			return db.GetUserSubscriptionRow{}, err
		}
	}
	return sub, nil
}

func (r *billingRepo) GetUserSubscriptionByStripeCustomerID(ctx context.Context, stripeCustomerID sql.NullString) (db.GetUserSubscriptionByStripeCustomerIDRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetUserSubscriptionByStripeCustomerID")
	defer span.End()

	return r.db.GetUserSubscriptionByStripeCustomerID(ctx, stripeCustomerID)
}

func (r *billingRepo) CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.UserSubscription, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CreateDefaultFreeSubscription")
	defer span.End()

	return r.db.CreateDefaultFreeSubscription(ctx, userID)
}

func (r *billingRepo) UpsertUserSubscription(ctx context.Context, params db.UpsertUserSubscriptionParams) (db.UserSubscription, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.UpsertUserSubscription")
	defer span.End()

	return r.db.UpsertUserSubscription(ctx, params)
}

func (r *billingRepo) CreateUpgradeEvent(ctx context.Context, params db.CreateUpgradeEventParams) (db.UpgradeEvent, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CreateUpgradeEvent")
	defer span.End()

	return r.db.CreateUpgradeEvent(ctx, params)
}

func (r *billingRepo) CountActiveGoalsForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CountActiveGoalsForUser")
	defer span.End()

	return r.db.CountActiveGoalsForUser(ctx, userID)
}

func (r *billingRepo) CountActiveHabitsForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CountActiveHabitsForUser")
	defer span.End()

	return r.db.CountActiveHabitsForUser(ctx, userID)
}

func (r *billingRepo) CountPendingPlanAdjustmentsForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CountPendingPlanAdjustmentsForUser")
	defer span.End()

	return r.db.CountPendingPlanAdjustmentsForUser(ctx, userID)
}

// EntitlementsResult holds computed entitlements for a user.
type EntitlementsResult struct {
	PlanCode                    string
	Status                      string
	ActiveGoalLimit             sql.NullInt32
	ActiveHabitLimit            sql.NullInt32
	WeeklyReviewHistoryLimit    sql.NullInt32
	PlanAdjustmentLimit         sql.NullInt32
	PersonalizedAiEnabled      bool
	CanCreateGoal               bool
	CanCreateHabit              bool
	CanViewWeeklyReviewHistory  bool
	CanUsePersonalizedAi        bool
	CanCreatePlanAdjustment     bool
	CurrentActiveGoals          int64
	CurrentActiveHabits         int64
	CurrentPendingAdjustments   int64
}

// ComputeEntitlements calculates what a user can do based on their plan and current usage.
func (r *billingRepo) ComputeEntitlements(ctx context.Context, sub db.GetUserSubscriptionRow, userID uuid.UUID) (*EntitlementsResult, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.ComputeEntitlements")
	defer span.End()

	activeGoals, err := r.CountActiveGoalsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	activeHabits, err := r.CountActiveHabitsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	pendingAdjustments, err := r.CountPendingPlanAdjustmentsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	isPro := sub.PlanCode == "pro" && (sub.Status == db.SubscriptionStatusTypeActive || sub.Status == db.SubscriptionStatusTypeTrialing)

	canCreateGoal := isPro || (!sub.ActiveGoalLimit.Valid || activeGoals < int64(sub.ActiveGoalLimit.Int32))
	canCreateHabit := isPro || (!sub.ActiveHabitLimit.Valid || activeHabits < int64(sub.ActiveHabitLimit.Int32))
	canViewHistory := isPro || !sub.WeeklyReviewHistoryLimit.Valid || sub.WeeklyReviewHistoryLimit.Int32 > 1
	canUsePersonalizedAi := isPro || sub.PersonalizedAiEnabled
	canCreatePlanAdjustment := isPro || (!sub.PlanAdjustmentLimit.Valid || pendingAdjustments < int64(sub.PlanAdjustmentLimit.Int32))

	return &EntitlementsResult{
		PlanCode:                   sub.PlanCode,
		Status:                     string(sub.Status),
		ActiveGoalLimit:            sub.ActiveGoalLimit,
		ActiveHabitLimit:           sub.ActiveHabitLimit,
		WeeklyReviewHistoryLimit:   sub.WeeklyReviewHistoryLimit,
		PlanAdjustmentLimit:        sub.PlanAdjustmentLimit,
		PersonalizedAiEnabled:      sub.PersonalizedAiEnabled,
		CanCreateGoal:              canCreateGoal,
		CanCreateHabit:             canCreateHabit,
		CanViewWeeklyReviewHistory: canViewHistory,
		CanUsePersonalizedAi:       canUsePersonalizedAi,
		CanCreatePlanAdjustment:    canCreatePlanAdjustment,
		CurrentActiveGoals:         activeGoals,
		CurrentActiveHabits:        activeHabits,
		CurrentPendingAdjustments:  pendingAdjustments,
	}, nil
}

// NullStringPtr returns a sql.NullString from a string pointer.
func NullStringPtr(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// NullJSON returns json.RawMessage for metadata.
func NullJSON(m map[string]interface{}) json.RawMessage {
	if m == nil {
		return json.RawMessage("{}")
	}
	b, _ := json.Marshal(m)
	return b
}
