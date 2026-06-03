package ai

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricNamespace = "ai"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "requests_total",
			Help:      "Total number of AI requests.",
		},
		[]string{"profile", "model", "status"},
	)

	tokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "tokens_total",
			Help:      "Total number of tokens processed.",
		},
		[]string{"profile", "model", "direction"},
	)

	costUSDTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "cost_usd_total",
			Help:      "Total estimated cost in USD.",
		},
		[]string{"profile"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "request_duration_seconds",
			Help:      "AI request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"profile", "model"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, tokensTotal, costUSDTotal, requestDuration)
}

// recordMetrics records Prometheus metrics for a call.
func recordMetrics(profile ModelProfile, modelID, status string, usage Usage, costUSD float64, latencyMS int64) {
	requestsTotal.WithLabelValues(string(profile), modelID, status).Inc()
	tokensTotal.WithLabelValues(string(profile), modelID, "prompt").Add(float64(usage.PromptTokens))
	tokensTotal.WithLabelValues(string(profile), modelID, "completion").Add(float64(usage.CompletionTokens))
	costUSDTotal.WithLabelValues(string(profile)).Add(costUSD)
	requestDuration.WithLabelValues(string(profile), modelID).Observe(float64(latencyMS) / 1000.0)
}
