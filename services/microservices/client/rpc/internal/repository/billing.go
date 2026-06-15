package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type billingRepo struct {
	db *db.Queries
}

func NewBillingRepo(queries *db.Queries) IBilling {
	return &billingRepo{db: queries}
}

// WithTx returns a new billingRepo backed by the given transaction.
func (r *billingRepo) WithTx(tx pgx.Tx) *billingRepo {
	return &billingRepo{db: r.db.WithTx(tx)}
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

	// Race-safe: use the atomic UPSERT instead of read-then-write.
	// ON CONFLICT handles the case where another concurrent request already inserted.
	_, err := r.db.CreateDefaultFreeSubscription(ctx, userID)
	if err != nil {
		return db.GetUserSubscriptionRow{}, err
	}

	// Always re-read so we get the fully-populated joined row (plan details included).
	sub, err := r.db.GetUserSubscription(ctx, userID)
	if err != nil {
		return db.GetUserSubscriptionRow{}, err
	}
	return sub, nil
}

func (r *billingRepo) GetUserSubscriptionByStripeCustomerID(ctx context.Context, stripeCustomerID *string) (db.GetUserSubscriptionByStripeCustomerIDRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetUserSubscriptionByStripeCustomerID")
	defer span.End()

	return r.db.GetUserSubscriptionByStripeCustomerID(ctx, stripeCustomerID)
}

func (r *billingRepo) CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.Subscription, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CreateDefaultFreeSubscription")
	defer span.End()

	return r.db.CreateDefaultFreeSubscription(ctx, userID)
}

func (r *billingRepo) UpsertUserSubscription(ctx context.Context, params db.UpsertUserSubscriptionParams) (db.Subscription, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.UpsertUserSubscription")
	defer span.End()

	return r.db.UpsertUserSubscription(ctx, params)
}

func (r *billingRepo) CreateUpgradeEvent(ctx context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error) {
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
	PlanCode                   string
	Status                     string
	ActiveGoalLimit            int32
	ActiveHabitLimit           int32
	WeeklyReviewHistoryLimit   int32
	PlanAdjustmentLimit        int32
	PersonalizedAiEnabled      bool
	CanCreateGoal              bool
	CanCreateHabit             bool
	CanViewWeeklyReviewHistory bool
	CanUsePersonalizedAi       bool
	CanCreatePlanAdjustment    bool
	CurrentActiveGoals         int64
	CurrentActiveHabits        int64
	CurrentPendingAdjustments  int64
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

	// past_due retains pro benefits during Stripe's grace period until explicit cancellation.
	isPro := sub.PlanCode == "pro" && (sub.Status == "active" || sub.Status == "trialing" || sub.Status == "past_due")

	canCreateGoal := isPro || activeGoals < int64(sub.ActiveGoalLimit)
	canCreateHabit := isPro || activeHabits < int64(sub.ActiveHabitLimit)
	canViewHistory := isPro || sub.WeeklyReviewHistoryLimit > 1
	canUsePersonalizedAi := isPro || sub.PersonalizedAiEnabled
	canCreatePlanAdjustment := isPro || pendingAdjustments < int64(sub.PlanAdjustmentLimit)

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

func (r *billingRepo) IsStripeEventProcessed(ctx context.Context, stripeEventID string) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.IsStripeEventProcessed")
	defer span.End()

	return r.db.IsStripeEventProcessed(ctx, stripeEventID)
}

func (r *billingRepo) MarkStripeEventProcessed(ctx context.Context, stripeEventID string) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.MarkStripeEventProcessed")
	defer span.End()

	return r.db.MarkStripeEventProcessed(ctx, stripeEventID)
}

func (r *billingRepo) ListExpiredActiveSubscriptions(ctx context.Context, limit int32) ([]db.ListExpiredActiveSubscriptionsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.ListExpiredActiveSubscriptions")
	defer span.End()

	return r.db.ListExpiredActiveSubscriptions(ctx, limit)
}

// NullJSON returns json.RawMessage for metadata.
func NullJSON(m map[string]interface{}) json.RawMessage {
	if m == nil {
		return json.RawMessage("{}")
	}
	b, _ := json.Marshal(m)
	return b
}
