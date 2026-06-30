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

	// promptTokensByFeature observes the prompt-token count of each AI call
	// labeled by feature (e.g. "personalized_coaching_stream"). This lets us
	// confirm prompt budgets hold per feature and tune them with real data.
	promptTokensByFeature = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "prompt_tokens",
			Help:      "Prompt token count per AI call, by feature.",
			Buckets:   []float64{128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768},
		},
		[]string{"feature"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, tokensTotal, costUSDTotal, requestDuration, promptTokensByFeature)
}

// recordMetrics records Prometheus metrics for a call. feature is the
// caller-supplied feature label from Metadata.Feature (may be empty).
func recordMetrics(profile ModelProfile, modelID, status, feature string, usage Usage, costUSD float64, latencyMS int64) {
	requestsTotal.WithLabelValues(string(profile), modelID, status).Inc()
	tokensTotal.WithLabelValues(string(profile), modelID, "prompt").Add(float64(usage.PromptTokens))
	tokensTotal.WithLabelValues(string(profile), modelID, "completion").Add(float64(usage.CompletionTokens))
	costUSDTotal.WithLabelValues(string(profile)).Add(costUSD)
	requestDuration.WithLabelValues(string(profile), modelID).Observe(float64(latencyMS) / 1000.0)
	if usage.PromptTokens > 0 {
		promptTokensByFeature.WithLabelValues(feature).Observe(float64(usage.PromptTokens))
	}
}
