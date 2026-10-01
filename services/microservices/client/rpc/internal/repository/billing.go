package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type billingRepo struct {
	db              *db.Queries
	habits          IHabits
	goals           IGoals
	planAdjustments IPlanAdjustmentSuggestions
}

func NewBillingRepo(queries *db.Queries, habits IHabits, goals IGoals, planAdjustments IPlanAdjustmentSuggestions) IBilling {
	return &billingRepo{db: queries, habits: habits, goals: goals, planAdjustments: planAdjustments}
}

// WithTx returns a new billingRepo backed by the given transaction.
func (r *billingRepo) WithTx(tx pgx.Tx) *billingRepo {
	return &billingRepo{
		db:              r.db.WithTx(tx),
		habits:          r.habits,
		goals:           r.goals,
		planAdjustments: r.planAdjustments,
	}
}

func (r *billingRepo) ListActivePlans(ctx context.Context) ([]db.Plan, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.ListActivePlans")
	defer span.End()

	return r.db.ListActivePlans(ctx)
}

func (r *billingRepo) ListSubscriptionStatuses(ctx context.Context) ([]db.ListSubscriptionStatusesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.ListSubscriptionStatuses")
	defer span.End()

	return r.db.ListSubscriptionStatuses(ctx)
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

func (r *billingRepo) CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.Subscription, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CreateDefaultFreeSubscription")
	defer span.End()

	return r.db.CreateDefaultFreeSubscription(ctx, userID)
}

func (r *billingRepo) ApplyMergedSubscription(ctx context.Context, params db.ApplyMergedSubscriptionParams) (db.Subscription, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.ApplyMergedSubscription")
	defer span.End()

	return r.db.ApplyMergedSubscription(ctx, params)
}

func (r *billingRepo) GetSubscriptionProviderState(ctx context.Context, userID uuid.UUID, provider string) (db.SubscriptionProviderState, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetSubscriptionProviderState")
	defer span.End()

	return r.db.GetSubscriptionProviderState(ctx, userID, provider)
}

func (r *billingRepo) ListSubscriptionProviderStates(ctx context.Context, userID uuid.UUID) ([]db.SubscriptionProviderState, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.ListSubscriptionProviderStates")
	defer span.End()

	return r.db.ListSubscriptionProviderStates(ctx, userID)
}

func (r *billingRepo) UpsertSubscriptionProviderState(ctx context.Context, params db.UpsertSubscriptionProviderStateParams) (db.SubscriptionProviderState, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.UpsertSubscriptionProviderState")
	defer span.End()

	return r.db.UpsertSubscriptionProviderState(ctx, params)
}

func (r *billingRepo) LinkPaddleProviderIDs(ctx context.Context, userID uuid.UUID, providerCustomerID, providerSubscriptionID *string, lastEventAt pgtype.Timestamptz) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.LinkPaddleProviderIDs")
	defer span.End()

	return r.db.LinkPaddleProviderIDs(ctx, userID, providerCustomerID, providerSubscriptionID, lastEventAt)
}

func (r *billingRepo) RecordPaddleCheckout(ctx context.Context, transactionID string, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.RecordPaddleCheckout")
	defer span.End()

	return r.db.RecordPaddleCheckout(ctx, transactionID, userID)
}

func (r *billingRepo) GetPaddleCheckoutUserID(ctx context.Context, transactionID string) (uuid.UUID, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetPaddleCheckoutUserID")
	defer span.End()

	return r.db.GetPaddleCheckoutUserID(ctx, transactionID)
}

func (r *billingRepo) CreateUpgradeEvent(ctx context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.CreateUpgradeEvent")
	defer span.End()

	return r.db.CreateUpgradeEvent(ctx, params)
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

	activeGoals, err := r.goals.CountActiveGoalsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	activeHabits, err := r.habits.CountHabitsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	pendingAdjustments, err := r.planAdjustments.CountPendingPlanAdjustmentSuggestions(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Pro requires a granting status AND a period that hasn't lapsed. Without
	// the period check a lapsed subscription whose provider webhook never
	// arrived (or was dropped) would report Pro indefinitely.
	// past_due retains pro benefits during the payment grace period until the
	// paid-through date runs out.
	isPro := GrantsProAccess(sub.PlanCode, string(sub.Status), sub.CurrentPeriodEnd, time.Now())

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

// EntitlementsOrFreeFallback resolves a user's entitlements without ever
// silently disabling limit enforcement. When the subscription row cannot be
// loaded, it falls back to the "free" plan's limits (bounded degradation):
// a billing failure can make a Pro user look free, but can never grant
// unlimited access. Returns an error only when even the fallback cannot be
// computed — callers must treat that as enforcement-failed and fail closed.
func (r *billingRepo) EntitlementsOrFreeFallback(ctx context.Context, userID uuid.UUID) (*EntitlementsResult, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.EntitlementsOrFreeFallback")
	defer span.End()

	sub, err := r.GetOrCreateUserSubscription(ctx, userID)
	if err == nil {
		return r.ComputeEntitlements(ctx, sub, userID)
	}

	plan, perr := r.db.GetPlanByCode(ctx, "free")
	if perr != nil {
		return nil, err
	}
	return r.ComputeEntitlements(ctx, db.GetUserSubscriptionRow{
		PlanCode:                 plan.Code,
		ActiveGoalLimit:          plan.ActiveGoalLimit,
		ActiveHabitLimit:         plan.ActiveHabitLimit,
		WeeklyReviewHistoryLimit: plan.WeeklyReviewHistoryLimit,
		PlanAdjustmentLimit:      plan.PlanAdjustmentLimit,
		PersonalizedAiEnabled:    plan.PersonalizedAiEnabled,
	}, userID)
}

// IsGrantingStatus reports whether a subscription status is in the class
// that can grant Pro (subject to the period check). 'paused' is deliberately
// excluded: a paused subscription stops paid access until it resumes.
func IsGrantingStatus(status string) bool {
	switch status {
	case "active", "trialing", "past_due":
		return true
	default:
		return false
	}
}

// GrantsProAccess reports whether a (plan, status, period) triple currently
// entitles the user to Pro. The period check is what keeps a lapsed
// subscription from reporting Pro while a missed expiry webhook is pending
// (billing correctness B3).
func GrantsProAccess(planCode, status string, periodEnd pgtype.Timestamptz, now time.Time) bool {
	return planCode == "pro" &&
		IsGrantingStatus(status) &&
		periodEnd.Valid && periodEnd.Time.After(now)
}

// NullStringPtr returns a sql.NullString from a string pointer.
func NullStringPtr(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// ─── RevenueCat ──────────────────────────────────────────────────────────────

func (r *billingRepo) GetUserSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionByUserIDRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetUserSubscriptionByUserID")
	defer span.End()
	return r.db.GetUserSubscriptionByUserID(ctx, userID)
}

func (r *billingRepo) IsRevenueCatEventProcessed(ctx context.Context, eventID string) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.IsRevenueCatEventProcessed")
	defer span.End()
	return r.db.IsRevenueCatEventProcessed(ctx, eventID)
}

func (r *billingRepo) MarkRevenueCatEventProcessed(ctx context.Context, eventID string) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.MarkRevenueCatEventProcessed")
	defer span.End()
	return r.db.MarkRevenueCatEventProcessed(ctx, eventID)
}

// ─── Paddle ──────────────────────────────────────────────────────────────────

func (r *billingRepo) GetUserSubscriptionByPaddleCustomerID(ctx context.Context, paddleCustomerID *string) (db.GetUserSubscriptionByPaddleCustomerIDRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.GetUserSubscriptionByPaddleCustomerID")
	defer span.End()
	return r.db.GetUserSubscriptionByPaddleCustomerID(ctx, paddleCustomerID)
}

func (r *billingRepo) IsPaddleEventProcessed(ctx context.Context, eventID string) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.IsPaddleEventProcessed")
	defer span.End()
	return r.db.IsPaddleEventProcessed(ctx, eventID)
}

func (r *billingRepo) MarkPaddleEventProcessed(ctx context.Context, eventID string) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "BillingRepo.MarkPaddleEventProcessed")
	defer span.End()
	return r.db.MarkPaddleEventProcessed(ctx, eventID)
}

// NullJSON returns json.RawMessage for metadata.
func NullJSON(m map[string]interface{}) json.RawMessage {
	if m == nil {
		return json.RawMessage("{}")
	}
	b, _ := json.Marshal(m)
	return b
}
