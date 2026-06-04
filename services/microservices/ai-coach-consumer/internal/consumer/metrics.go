package consumer

import (
	"github.com/prometheus/client_golang/prometheus"
)

const metricNamespace = "consumer"

var (
	eventsConsumedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "events_consumed_total",
			Help:      "Total events consumed from Kafka.",
		},
		[]string{"event_type", "status"},
	)

	eventsProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "event_processing_duration_seconds",
			Help:      "Event processing duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	eventsDuplicateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "events_duplicate_total",
			Help:      "Total duplicate events skipped.",
		},
		[]string{"event_type"},
	)

	eventsDLQTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "events_dlq_total",
			Help:      "Total events sent to the dead-letter queue.",
		},
		[]string{"event_type", "reason"},
	)

	aiGenerationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "ai_generation_duration_seconds",
			Help:      "AI generation call duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	aiSafetyBlockedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "ai_safety_blocked_total",
			Help:      "Total events blocked by the safety classifier.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		eventsConsumedTotal,
		eventsProcessingDuration,
		eventsDuplicateTotal,
		eventsDLQTotal,
		aiGenerationDuration,
		aiSafetyBlockedTotal,
	)
}
