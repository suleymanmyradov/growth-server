package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriptionStatusType mirrors the Postgres enum.
type SubscriptionStatusType string

const (
	SubscriptionStatusTypeActive   SubscriptionStatusType = "active"
	SubscriptionStatusTypeTrialing SubscriptionStatusType = "trialing"
	SubscriptionStatusTypeExpired  SubscriptionStatusType = "expired"
	SubscriptionStatusTypePastDue  SubscriptionStatusType = "past_due"
	SubscriptionStatusTypeFree   SubscriptionStatusType = "free"
)

// BillingIntervalType mirrors the Postgres enum.
type BillingIntervalType string

const (
	BillingIntervalTypeMonthly BillingIntervalType = "monthly"
	BillingIntervalTypeAnnual  BillingIntervalType = "annual"
)

// Plan mirrors the plans table.
type Plan struct {
	ID                       uuid.UUID          `db:"id" json:"id"`
	Code                     string             `db:"code" json:"code"`
	Name                     string             `db:"name" json:"name"`
	Description              *string            `db:"description" json:"description"`
	PriceMonthlyCents        int32              `db:"price_monthly_cents" json:"price_monthly_cents"`
	PriceAnnualCents         int32              `db:"price_annual_cents" json:"price_annual_cents"`
	ActiveGoalLimit          int32              `db:"active_goal_limit" json:"active_goal_limit"`
	ActiveHabitLimit         int32              `db:"active_habit_limit" json:"active_habit_limit"`
	WeeklyReviewHistoryLimit int32              `db:"weekly_review_history_limit" json:"weekly_review_history_limit"`
	PlanAdjustmentLimit      int32              `db:"plan_adjustment_limit" json:"plan_adjustment_limit"`
	PersonalizedAiEnabled    bool               `db:"personalized_ai_enabled" json:"personalized_ai_enabled"`
	StripeMonthlyPriceID     *string            `db:"stripe_monthly_price_id" json:"stripe_monthly_price_id"`
	StripeAnnualPriceID      *string            `db:"stripe_annual_price_id" json:"stripe_annual_price_id"`
	IsActive                 bool               `db:"is_active" json:"is_active"`
	CreatedAt                pgtype.Timestamptz `db:"created_at" json:"created_at"`
	UpdatedAt                pgtype.Timestamptz `db:"updated_at" json:"updated_at"`
	Currency                 string             `db:"currency" json:"currency"`
}

// UserSubscription mirrors the subscriptions table.
type UserSubscription struct {
	ID                   uuid.UUID              `db:"id" json:"id"`
	UserID               uuid.UUID              `db:"user_id" json:"user_id"`
	PlanID               uuid.UUID              `db:"plan_id" json:"plan_id"`
	Status               SubscriptionStatusType `db:"status" json:"status"`
	BillingInterval      *BillingIntervalType   `db:"billing_interval" json:"billing_interval"`
	CurrentPeriodStart   pgtype.Timestamptz     `db:"current_period_start" json:"current_period_start"`
	CurrentPeriodEnd     pgtype.Timestamptz     `db:"current_period_end" json:"current_period_end"`
	TrialEnd             pgtype.Timestamptz     `db:"trial_end" json:"trial_end"`
	CancelAtPeriodEnd    bool                   `db:"cancel_at_period_end" json:"cancel_at_period_end"`
	StripeCustomerID     *string                `db:"stripe_customer_id" json:"stripe_customer_id"`
	StripeSubscriptionID *string                `db:"stripe_subscription_id" json:"stripe_subscription_id"`
	CreatedAt            pgtype.Timestamptz     `db:"created_at" json:"created_at"`
	UpdatedAt            pgtype.Timestamptz     `db:"updated_at" json:"updated_at"`
}

// ListExpiredActiveSubscriptionsRow is the result of the expired-subscriptions query.
type ListExpiredActiveSubscriptionsRow struct {
	ID                       uuid.UUID              `db:"id" json:"id"`
	UserID                   uuid.UUID              `db:"user_id" json:"user_id"`
	PlanID                   uuid.UUID              `db:"plan_id" json:"plan_id"`
	Status                   SubscriptionStatusType `db:"status" json:"status"`
	BillingInterval          *BillingIntervalType   `db:"billing_interval" json:"billing_interval"`
	CurrentPeriodStart       pgtype.Timestamptz     `db:"current_period_start" json:"current_period_start"`
	CurrentPeriodEnd         pgtype.Timestamptz     `db:"current_period_end" json:"current_period_end"`
	TrialEnd                 pgtype.Timestamptz     `db:"trial_end" json:"trial_end"`
	CancelAtPeriodEnd        bool                   `db:"cancel_at_period_end" json:"cancel_at_period_end"`
	StripeCustomerID         *string                `db:"stripe_customer_id" json:"stripe_customer_id"`
	StripeSubscriptionID     *string                `db:"stripe_subscription_id" json:"stripe_subscription_id"`
	CreatedAt                pgtype.Timestamptz     `db:"created_at" json:"created_at"`
	UpdatedAt                pgtype.Timestamptz     `db:"updated_at" json:"updated_at"`
	PlanCode                 string                 `db:"plan_code" json:"plan_code"`
	PlanName                 string                 `db:"plan_name" json:"plan_name"`
	ActiveGoalLimit          int32                  `db:"active_goal_limit" json:"active_goal_limit"`
	ActiveHabitLimit         int32                  `db:"active_habit_limit" json:"active_habit_limit"`
	WeeklyReviewHistoryLimit int32                  `db:"weekly_review_history_limit" json:"weekly_review_history_limit"`
	PlanAdjustmentLimit      int32                  `db:"plan_adjustment_limit" json:"plan_adjustment_limit"`
	PersonalizedAiEnabled    bool                   `db:"personalized_ai_enabled" json:"personalized_ai_enabled"`
}

// UpsertUserSubscriptionParams is the input for the upsert query.
type UpsertUserSubscriptionParams struct {
	UserID               uuid.UUID              `db:"user_id" json:"user_id"`
	PlanID               uuid.UUID              `db:"plan_id" json:"plan_id"`
	Status               SubscriptionStatusType `db:"status" json:"status"`
	BillingInterval      *BillingIntervalType   `db:"billing_interval" json:"billing_interval"`
	CurrentPeriodStart   pgtype.Timestamptz     `db:"current_period_start" json:"current_period_start"`
	CurrentPeriodEnd     pgtype.Timestamptz     `db:"current_period_end" json:"current_period_end"`
	TrialEnd             pgtype.Timestamptz     `db:"trial_end" json:"trial_end"`
	CancelAtPeriodEnd    bool                   `db:"cancel_at_period_end" json:"cancel_at_period_end"`
	StripeCustomerID     *string                `db:"stripe_customer_id" json:"stripe_customer_id"`
	StripeSubscriptionID *string                `db:"stripe_subscription_id" json:"stripe_subscription_id"`
}

// BillingRepository performs billing-related DB queries.
type BillingRepository struct {
	pool *pgxpool.Pool
}

func NewBillingRepository(pool *pgxpool.Pool) *BillingRepository {
	return &BillingRepository{pool: pool}
}

func (r *BillingRepository) ListExpiredActiveSubscriptions(ctx context.Context, limit int32) ([]ListExpiredActiveSubscriptionsRow, error) {
	query := `
	SELECT
	    us.id, us.user_id, us.plan_id, us.status, us.billing_interval, us.current_period_start, us.current_period_end, us.trial_end, us.cancel_at_period_end, us.stripe_customer_id, us.stripe_subscription_id, us.created_at, us.updated_at,
	    p.code AS plan_code,
	    p.name AS plan_name,
	    p.active_goal_limit,
	    p.active_habit_limit,
	    p.weekly_review_history_limit,
	    p.plan_adjustment_limit,
	    p.personalized_ai_enabled
	FROM subscriptions us
	JOIN plans p ON p.id = us.plan_id
	WHERE us.status IN ('active', 'trialing')
	  AND us.cancel_at_period_end = true
	  AND us.current_period_end < NOW()
	LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query expired subscriptions: %w", err)
	}
	defer rows.Close()

	var result []ListExpiredActiveSubscriptionsRow
	for rows.Next() {
		var i ListExpiredActiveSubscriptionsRow
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.PlanID,
			&i.Status,
			&i.BillingInterval,
			&i.CurrentPeriodStart,
			&i.CurrentPeriodEnd,
			&i.TrialEnd,
			&i.CancelAtPeriodEnd,
			&i.StripeCustomerID,
			&i.StripeSubscriptionID,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PlanCode,
			&i.PlanName,
			&i.ActiveGoalLimit,
			&i.ActiveHabitLimit,
			&i.WeeklyReviewHistoryLimit,
			&i.PlanAdjustmentLimit,
			&i.PersonalizedAiEnabled,
		); err != nil {
			return nil, fmt.Errorf("scan expired subscription: %w", err)
		}
		result = append(result, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired subscriptions: %w", err)
	}
	return result, nil
}

func (r *BillingRepository) GetPlanByCode(ctx context.Context, code string) (Plan, error) {
	query := `
	SELECT id, code, name, description, price_monthly_cents, price_annual_cents, active_goal_limit, active_habit_limit, weekly_review_history_limit, plan_adjustment_limit, personalized_ai_enabled, stripe_monthly_price_id, stripe_annual_price_id, is_active, created_at, updated_at, currency
	FROM plans
	WHERE code = $1 AND is_active = TRUE`

	var i Plan
	row := r.pool.QueryRow(ctx, query, code)
	err := row.Scan(
		&i.ID,
		&i.Code,
		&i.Name,
		&i.Description,
		&i.PriceMonthlyCents,
		&i.PriceAnnualCents,
		&i.ActiveGoalLimit,
		&i.ActiveHabitLimit,
		&i.WeeklyReviewHistoryLimit,
		&i.PlanAdjustmentLimit,
		&i.PersonalizedAiEnabled,
		&i.StripeMonthlyPriceID,
		&i.StripeAnnualPriceID,
		&i.IsActive,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Currency,
	)
	if err != nil {
		return Plan{}, fmt.Errorf("get plan by code: %w", err)
	}
	return i, nil
}

func (r *BillingRepository) UpsertUserSubscription(ctx context.Context, arg UpsertUserSubscriptionParams) (UserSubscription, error) {
	query := `
	INSERT INTO subscriptions (
	    user_id,
	    plan_id,
	    status,
	    billing_interval,
	    current_period_start,
	    current_period_end,
	    trial_end,
	    cancel_at_period_end,
	    stripe_customer_id,
	    stripe_subscription_id
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (user_id)
	DO UPDATE SET
	    plan_id = EXCLUDED.plan_id,
	    status = EXCLUDED.status,
	    billing_interval = EXCLUDED.billing_interval,
	    current_period_start = EXCLUDED.current_period_start,
	    current_period_end = EXCLUDED.current_period_end,
	    trial_end = EXCLUDED.trial_end,
	    cancel_at_period_end = EXCLUDED.cancel_at_period_end,
	    stripe_customer_id = EXCLUDED.stripe_customer_id,
	    stripe_subscription_id = EXCLUDED.stripe_subscription_id,
	    updated_at = CURRENT_TIMESTAMP
	RETURNING id, user_id, plan_id, status, billing_interval, current_period_start, current_period_end, trial_end, cancel_at_period_end, stripe_customer_id, stripe_subscription_id, created_at, updated_at`

	var i UserSubscription
	row := r.pool.QueryRow(ctx, query,
		arg.UserID,
		arg.PlanID,
		arg.Status,
		arg.BillingInterval,
		arg.CurrentPeriodStart,
		arg.CurrentPeriodEnd,
		arg.TrialEnd,
		arg.CancelAtPeriodEnd,
		arg.StripeCustomerID,
		arg.StripeSubscriptionID,
	)
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.PlanID,
		&i.Status,
		&i.BillingInterval,
		&i.CurrentPeriodStart,
		&i.CurrentPeriodEnd,
		&i.TrialEnd,
		&i.CancelAtPeriodEnd,
		&i.StripeCustomerID,
		&i.StripeSubscriptionID,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	if err != nil {
		return UserSubscription{}, fmt.Errorf("upsert subscription: %w", err)
	}
	return i, nil
}
