package billingservicelogic

import "github.com/prometheus/client_golang/prometheus"

// Billing webhook observability. The permanent-failure counter is the alert
// source: those events are marked processed and silently skipped after one
// Errorf line, so without a metric nobody notices revenue events being
// dropped. Retryable failures roll back and resurface as gRPC errors instead.
var billingWebhookPermanentFailuresTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "client",
		Name:      "billing_webhook_permanent_failures_total",
		Help:      "Billing webhook events permanently failed and marked processed (not retried).",
	},
	[]string{"provider", "event_type"},
)

func init() {
	prometheus.MustRegister(billingWebhookPermanentFailuresTotal)
}
