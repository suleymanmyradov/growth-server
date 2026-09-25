package paddle

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event types the backend acts on. Anything else parses fine into Envelope —
// inspect EventType and ignore or decode Data ad hoc.
const (
	EventTransactionCompleted = "transaction.completed"
	EventSubscriptionCreated  = "subscription.created"
	EventSubscriptionUpdated  = "subscription.updated"
	EventSubscriptionCanceled = "subscription.canceled"
	EventSubscriptionPastDue  = "subscription.past_due"
)

// Envelope is the wrapper Paddle puts around every webhook notification.
// Data is left raw; decode it with Transaction or Subscription depending on
// EventType, or leave it for event types we don't model.
type Envelope struct {
	// EventID is the unique event identifier (evt_...). Use it for
	// idempotency — Paddle retries deliveries.
	EventID string `json:"event_id"`
	// EventType is the dotted event name, e.g. "subscription.updated".
	EventType string `json:"event_type"`
	// OccurredAt is when the event happened in Paddle.
	OccurredAt time.Time `json:"occurred_at"`
	// NotificationID is the delivery identifier (ntf_...). Retried deliveries
	// of the same event share the EventID but have distinct notification IDs.
	NotificationID string `json:"notification_id"`
	// Data is the raw entity payload (transaction, subscription, ...).
	Data json.RawMessage `json:"data"`
}

// ParseEvent decodes the webhook body into an Envelope. It does NOT verify
// the signature — call VerifySignature (or a Verifier) first so unauthenticated
// payloads are rejected before parsing.
func ParseEvent(body []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("paddle: parse event: %w", err)
	}
	if env.EventType == "" {
		return nil, fmt.Errorf("paddle: event missing event_type")
	}
	return &env, nil
}

// Transaction decodes Data as a transaction entity. Valid for
// transaction.* events.
func (e *Envelope) Transaction() (*Transaction, error) {
	var txn Transaction
	if err := json.Unmarshal(e.Data, &txn); err != nil {
		return nil, fmt.Errorf("paddle: decode transaction data: %w", err)
	}
	return &txn, nil
}

// Subscription decodes Data as a subscription entity. Valid for
// subscription.* events.
func (e *Envelope) Subscription() (*Subscription, error) {
	var sub Subscription
	if err := json.Unmarshal(e.Data, &sub); err != nil {
		return nil, fmt.Errorf("paddle: decode subscription data: %w", err)
	}
	return &sub, nil
}

// UserID extracts custom_data.user_id from the raw event data — the internal
// user ID stamped by CreateTransaction. Empty when absent.
func (e *Envelope) UserID() string {
	var probe struct {
		CustomData map[string]any `json:"custom_data"`
	}
	if err := json.Unmarshal(e.Data, &probe); err != nil {
		return ""
	}
	return userIDFromCustomData(probe.CustomData)
}

// userIDFromCustomData pulls the "user_id" string out of a Paddle
// custom_data object.
func userIDFromCustomData(cd map[string]any) string {
	s, _ := cd["user_id"].(string)
	return s
}

// TransactionCheckout holds the payment link for an automatically-collected
// transaction.
type TransactionCheckout struct {
	// URL is the hosted payment link: the checkout URL passed at creation (or
	// the dashboard default payment link) + "?_ptxn=<transaction id>".
	URL string `json:"url"`
}

// TransactionItem is a line item on a transaction.
type TransactionItem struct {
	// PriceID is the catalog price (pri_...) being charged.
	PriceID string `json:"price_id"`
	// Quantity is the number of units.
	Quantity int `json:"quantity"`
}

// Transaction is a Paddle transaction entity, as returned by the API and
// carried in transaction.* webhook events. Only fields we consume are
// modeled; unknown fields are ignored.
type Transaction struct {
	// ID is the Paddle transaction ID (txn_...).
	ID string `json:"id"`
	// Status is draft|ready|billed|paid|completed|canceled|past_due.
	Status string `json:"status"`
	// CustomerID is the Paddle customer (ctm_...).
	CustomerID string `json:"customer_id"`
	// SubscriptionID is the subscription created by this transaction
	// (sub_...), set once the transaction is completed for a recurring item.
	SubscriptionID string `json:"subscription_id"`
	// CurrencyCode is the ISO 4217 transaction currency.
	CurrencyCode string `json:"currency_code"`
	// CollectionMode is automatic (checkout) or manual (invoice).
	CollectionMode string `json:"collection_mode"`
	// Origin describes how the transaction was created (e.g. "api", "web").
	Origin string `json:"origin"`
	// CustomData is the structured data passed at creation; user_id lives here.
	CustomData map[string]any `json:"custom_data"`
	// Items lists the charged line items.
	Items []TransactionItem `json:"items"`
	// Checkout holds the payment link; nil for manually-collected transactions
	// where checkout is not enabled.
	Checkout *TransactionCheckout `json:"checkout"`
	// CreatedAt is when the transaction was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the transaction was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// BilledAt is when the transaction was billed; nil if not billed.
	BilledAt *time.Time `json:"billed_at"`
}

// UserID extracts custom_data.user_id — the internal user ID stamped by
// CreateTransaction. Empty when absent.
func (t *Transaction) UserID() string { return userIDFromCustomData(t.CustomData) }

// CheckoutURL returns the hosted payment link, or "" if Paddle did not
// generate one.
func (t *Transaction) CheckoutURL() string {
	if t.Checkout == nil {
		return ""
	}
	return t.Checkout.URL
}

// BillingCycle describes how often a subscription or price bills.
type BillingCycle struct {
	// Interval is the unit of time: day|week|month|year.
	Interval string `json:"interval"`
	// Frequency is the number of intervals between billings (e.g. 1 month).
	Frequency int `json:"frequency"`
}

// BillingPeriod is the time range a charge covers.
type BillingPeriod struct {
	// StartsAt is the period start.
	StartsAt time.Time `json:"starts_at"`
	// EndsAt is the period end.
	EndsAt time.Time `json:"ends_at"`
}

// ScheduledChange is a pending subscription change (cancel, pause, resume).
type ScheduledChange struct {
	// Action is cancel|pause|resume.
	Action string `json:"action"`
	// EffectiveAt is when the change applies.
	EffectiveAt *time.Time `json:"effective_at"`
	// ResumeAt is when a paused subscription resumes; nil unless action=resume.
	ResumeAt *time.Time `json:"resume_at"`
}

// ItemPrice is the catalog price embedded in a subscription item.
type ItemPrice struct {
	// ID is the Paddle price ID (pri_...).
	ID string `json:"id"`
	// ProductID is the Paddle product ID (pro_...).
	ProductID string `json:"product_id"`
	// BillingCycle is the recurring interval of the price; nil for one-time.
	BillingCycle *BillingCycle `json:"billing_cycle"`
}

// SubscriptionItem is a line item on a subscription.
type SubscriptionItem struct {
	// Status is the item status (active|trialing|inactive).
	Status string `json:"status"`
	// Quantity is the number of units.
	Quantity int `json:"quantity"`
	// Recurring is whether the item bills on a cycle.
	Recurring bool `json:"recurring"`
	// Price is the catalog price for the item.
	Price *ItemPrice `json:"price"`
}

// Subscription is a Paddle subscription entity, as returned by the API and
// carried in subscription.* webhook events. Only fields we consume are
// modeled; unknown fields are ignored.
type Subscription struct {
	// ID is the Paddle subscription ID (sub_...).
	ID string `json:"id"`
	// Status is active|trialing|past_due|paused|canceled.
	Status string `json:"status"`
	// CustomerID is the Paddle customer (ctm_...).
	CustomerID string `json:"customer_id"`
	// AddressID is the billing address (add_...).
	AddressID string `json:"address_id"`
	// CurrencyCode is the ISO 4217 subscription currency.
	CurrencyCode string `json:"currency_code"`
	// CollectionMode is automatic (charge saved payment method) or manual.
	CollectionMode string `json:"collection_mode"`
	// CustomData carries structured data set on the subscription; may be nil
	// even when the originating transaction had custom_data.
	CustomData map[string]any `json:"custom_data"`
	// Items lists the subscription line items.
	Items []SubscriptionItem `json:"items"`
	// BillingCycle is the top-level recurring interval.
	BillingCycle *BillingCycle `json:"billing_cycle"`
	// CurrentBillingPeriod is the period currently being billed for.
	CurrentBillingPeriod *BillingPeriod `json:"current_billing_period"`
	// ScheduledChange is a pending cancel/pause/resume; nil when none.
	ScheduledChange *ScheduledChange `json:"scheduled_change"`
	// StartedAt is when the subscription first became active.
	StartedAt *time.Time `json:"started_at"`
	// FirstBilledAt is when the subscription was first billed.
	FirstBilledAt *time.Time `json:"first_billed_at"`
	// NextBilledAt is the next scheduled billing; nil when canceled/paused.
	NextBilledAt *time.Time `json:"next_billed_at"`
	// PausedAt is when the subscription was paused; nil unless paused.
	PausedAt *time.Time `json:"paused_at"`
	// CanceledAt is when the subscription was canceled; nil unless canceled.
	CanceledAt *time.Time `json:"canceled_at"`
	// CreatedAt is when the subscription entity was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the subscription entity was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// UserID extracts custom_data.user_id. Empty when absent — note Paddle does
// not copy transaction custom_data onto the subscription it creates, so prefer
// resolving the user via CustomerID or the originating transaction.
func (s *Subscription) UserID() string { return userIDFromCustomData(s.CustomData) }

// CancelScheduled reports whether a cancel change is scheduled to take effect
// at the end of the current billing period.
func (s *Subscription) CancelScheduled() bool {
	return s.ScheduledChange != nil && s.ScheduledChange.Action == "cancel"
}

// PauseScheduled reports whether a pause change is scheduled to take effect
// at the end of the current billing period.
func (s *Subscription) PauseScheduled() bool {
	return s.ScheduledChange != nil && s.ScheduledChange.Action == "pause"
}

// PriceID returns the catalog price ID (pri_...) of the first subscription
// item, or "" when there are none.
func (s *Subscription) PriceID() string {
	if len(s.Items) == 0 || s.Items[0].Price == nil {
		return ""
	}
	return s.Items[0].Price.ID
}
