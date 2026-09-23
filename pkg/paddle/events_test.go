package paddle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const transactionCompletedBody = `{
	"event_id": "evt_01h1vjes1y163xfj1rh1tkfb65",
	"event_type": "transaction.completed",
	"occurred_at": "2024-04-11T15:57:24.813Z",
	"notification_id": "ntf_01h1vjes1y163xfj1rh1tkfb66",
	"data": {
		"id": "txn_01hgk505qd8mm8yt0yvk1b6w8x",
		"status": "completed",
		"customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk",
		"address_id": "add_01h848pep46enq8y372x7maj0p",
		"subscription_id": "sub_01h849vje9skebr9p3bx2g0mxn",
		"currency_code": "USD",
		"collection_mode": "automatic",
		"origin": "api",
		"custom_data": {"user_id": "0191fa87-6ed1-7022-9999-0123456789ae"},
		"items": [
			{"price_id": "pri_01gsz8x8sawmvhz1pv30nge1ke", "quantity": 1}
		],
		"checkout": {"url": "https://example.com/checkout?_ptxn=txn_01hgk505qd8mm8yt0yvk1b6w8x"},
		"created_at": "2024-04-11T15:56:00.000Z",
		"updated_at": "2024-04-11T15:57:24.800Z",
		"billed_at": "2024-04-11T15:57:24.800Z"
	}
}`

const subscriptionUpdatedBody = `{
	"event_id": "evt_01h8441jn5pcwrfhwh78jqt800",
	"event_type": "subscription.updated",
	"occurred_at": "2024-04-12T09:00:00.000Z",
	"notification_id": "ntf_01h8441jn5pcwrfhwh78jqt801",
	"data": {
		"id": "sub_01h849vje9skebr9p3bx2g0mxn",
		"status": "active",
		"customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk",
		"address_id": "add_01h848pep46enq8y372x7maj0p",
		"currency_code": "USD",
		"collection_mode": "automatic",
		"custom_data": null,
		"items": [
			{
				"status": "active",
				"quantity": 1,
				"recurring": true,
				"price": {
					"id": "pri_01gsz8x8sawmvhz1pv30nge1ke",
					"product_id": "pro_01gsz4t5hdjse780zja8vvr7jg",
					"billing_cycle": {"interval": "month", "frequency": 1}
				}
			}
		],
		"billing_cycle": {"interval": "month", "frequency": 1},
		"current_billing_period": {
			"starts_at": "2024-04-11T15:57:24.800Z",
			"ends_at": "2024-05-11T15:57:24.800Z"
		},
		"scheduled_change": null,
		"started_at": "2024-04-11T15:57:24.800Z",
		"first_billed_at": "2024-04-11T15:57:24.800Z",
		"next_billed_at": "2024-05-11T15:57:24.800Z",
		"paused_at": null,
		"canceled_at": null,
		"created_at": "2024-04-11T15:57:24.800Z",
		"updated_at": "2024-04-12T09:00:00.000Z"
	}
}`

const subscriptionCanceledBody = `{
	"event_id": "evt_01h8441jn5pcwrfhwh78jqt900",
	"event_type": "subscription.canceled",
	"occurred_at": "2024-05-11T15:57:24.800Z",
	"notification_id": "ntf_01h8441jn5pcwrfhwh78jqt901",
	"data": {
		"id": "sub_01h849vje9skebr9p3bx2g0mxn",
		"status": "canceled",
		"customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk",
		"currency_code": "USD",
		"collection_mode": "automatic",
		"items": [],
		"billing_cycle": {"interval": "month", "frequency": 1},
		"current_billing_period": {
			"starts_at": "2024-04-11T15:57:24.800Z",
			"ends_at": "2024-05-11T15:57:24.800Z"
		},
		"scheduled_change": {
			"action": "cancel",
			"effective_at": "2024-05-11T15:57:24.800Z",
			"resume_at": null
		},
		"started_at": "2024-04-11T15:57:24.800Z",
		"first_billed_at": "2024-04-11T15:57:24.800Z",
		"next_billed_at": null,
		"canceled_at": "2024-05-11T15:57:24.800Z",
		"created_at": "2024-04-11T15:57:24.800Z",
		"updated_at": "2024-05-11T15:57:24.800Z"
	}
}`

func TestParseEvent_TransactionCompleted(t *testing.T) {
	env, err := ParseEvent([]byte(transactionCompletedBody))
	require.NoError(t, err)
	assert.Equal(t, "evt_01h1vjes1y163xfj1rh1tkfb65", env.EventID)
	assert.Equal(t, EventTransactionCompleted, env.EventType)
	assert.Equal(t, "ntf_01h1vjes1y163xfj1rh1tkfb66", env.NotificationID)
	assert.Equal(t, 2024, env.OccurredAt.Year())

	txn, err := env.Transaction()
	require.NoError(t, err)
	assert.Equal(t, "txn_01hgk505qd8mm8yt0yvk1b6w8x", txn.ID)
	assert.Equal(t, "completed", txn.Status)
	assert.Equal(t, "ctm_01h8441jn5pcwrfhwh78jqt8hk", txn.CustomerID)
	assert.Equal(t, "sub_01h849vje9skebr9p3bx2g0mxn", txn.SubscriptionID)
	assert.Equal(t, "0191fa87-6ed1-7022-9999-0123456789ae", txn.UserID())
	assert.Equal(t, "0191fa87-6ed1-7022-9999-0123456789ae", env.UserID())
	assert.Equal(t, "https://example.com/checkout?_ptxn=txn_01hgk505qd8mm8yt0yvk1b6w8x", txn.CheckoutURL())
	require.Len(t, txn.Items, 1)
	assert.Equal(t, "pri_01gsz8x8sawmvhz1pv30nge1ke", txn.Items[0].PriceID)
}

func TestParseEvent_SubscriptionUpdated(t *testing.T) {
	env, err := ParseEvent([]byte(subscriptionUpdatedBody))
	require.NoError(t, err)
	assert.Equal(t, EventSubscriptionUpdated, env.EventType)

	sub, err := env.Subscription()
	require.NoError(t, err)
	assert.Equal(t, "sub_01h849vje9skebr9p3bx2g0mxn", sub.ID)
	assert.Equal(t, "active", sub.Status)
	assert.Equal(t, "ctm_01h8441jn5pcwrfhwh78jqt8hk", sub.CustomerID)
	assert.Equal(t, "pri_01gsz8x8sawmvhz1pv30nge1ke", sub.PriceID())
	require.NotNil(t, sub.BillingCycle)
	assert.Equal(t, "month", sub.BillingCycle.Interval)
	assert.Equal(t, 1, sub.BillingCycle.Frequency)
	require.NotNil(t, sub.CurrentBillingPeriod)
	assert.Equal(t, time.Month(5), sub.CurrentBillingPeriod.EndsAt.Month())
	assert.Nil(t, sub.ScheduledChange)
	assert.False(t, sub.CancelScheduled())
	// Paddle does not copy transaction custom_data onto subscriptions.
	assert.Empty(t, sub.UserID())
	assert.NotNil(t, sub.NextBilledAt)
	assert.Nil(t, sub.CanceledAt)
}

func TestParseEvent_SubscriptionCanceled(t *testing.T) {
	env, err := ParseEvent([]byte(subscriptionCanceledBody))
	require.NoError(t, err)
	assert.Equal(t, EventSubscriptionCanceled, env.EventType)

	sub, err := env.Subscription()
	require.NoError(t, err)
	assert.Equal(t, "canceled", sub.Status)
	assert.True(t, sub.CancelScheduled())
	require.NotNil(t, sub.ScheduledChange)
	assert.Equal(t, "cancel", sub.ScheduledChange.Action)
	assert.Nil(t, sub.NextBilledAt)
	assert.NotNil(t, sub.CanceledAt)
	assert.Empty(t, sub.PriceID())
}

func TestParseEvent_SubscriptionPastDue(t *testing.T) {
	body := `{
		"event_id": "evt_pastdue01",
		"event_type": "subscription.past_due",
		"occurred_at": "2024-05-12T15:57:24.800Z",
		"notification_id": "ntf_pastdue01",
		"data": {"id": "sub_01h849vje9skebr9p3bx2g0mxn", "status": "past_due", "customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk"}
	}`
	env, err := ParseEvent([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, EventSubscriptionPastDue, env.EventType)

	sub, err := env.Subscription()
	require.NoError(t, err)
	assert.Equal(t, "past_due", sub.Status)
}

func TestParseEvent_UnknownTypeKeepsRawData(t *testing.T) {
	// Events we don't model still parse into the generic envelope.
	body := `{
		"event_id": "evt_prod01",
		"event_type": "product.updated",
		"occurred_at": "2024-04-11T15:57:24.813Z",
		"notification_id": "ntf_prod01",
		"data": {"id": "pro_01gsz4t5hdjse780zja8vvr7jg", "name": "Growth Pro"}
	}`
	env, err := ParseEvent([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, "product.updated", env.EventType)
	assert.Contains(t, string(env.Data), "Growth Pro")
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	_, err := ParseEvent([]byte(`{not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse event")
}

func TestParseEvent_MissingEventType(t *testing.T) {
	_, err := ParseEvent([]byte(`{"event_id":"evt_x","data":{}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event_type")
}

func TestEnvelope_UserIDAbsent(t *testing.T) {
	env, err := ParseEvent([]byte(subscriptionUpdatedBody))
	require.NoError(t, err)
	assert.Empty(t, env.UserID())

	// Malformed data must not panic.
	env = &Envelope{Data: []byte(`{`)}
	assert.Empty(t, env.UserID())
}
