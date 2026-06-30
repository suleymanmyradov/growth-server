package aicoachservicelogic

import (
	"github.com/prometheus/client_golang/prometheus"
)

const metricNamespace = "ai_coach"

var (
	// coachingSafetyBlockedTotal counts coaching requests blocked by the
	// safety classifier before reaching the model, labelled by the verdict
	// category (crisis / self_harm / medical / violence).
	coachingSafetyBlockedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "coaching_safety_blocked_total",
			Help:      "Total coaching requests blocked by the safety classifier.",
		},
		[]string{"category"},
	)

	// coachingSafetyClassifyErrors counts failures of the safety classifier
	// itself (e.g. model outage). The coaching path fails open on these, so
	// this metric makes outages observable.
	coachingSafetyClassifyErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Name:      "coaching_safety_classify_errors_total",
			Help:      "Total safety classifier errors on the coaching path (fail-open).",
		},
	)

	// coachingPromptSectionTokens observes the token cost of each personalization
	// section, labeled by section name and whether it was included or dropped by
	// the budget assembler. This is the per-section breakdown that lets us
	// confirm the budget holds and tune priority tiers with real data.
	coachingPromptSectionTokens = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "coaching_prompt_section_tokens",
			Help:      "Token cost of each coaching prompt section, by section and inclusion.",
			Buckets:   []float64{16, 32, 64, 128, 256, 512, 1024, 2048},
		},
		[]string{"section", "included"},
	)

	// coachingContextTokens observes the total token cost of the assembled
	// context sections (excluding the mandatory user message) per feature, so
	// we can confirm the budget holds end-to-end.
	coachingContextTokens = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Name:      "coaching_context_tokens",
			Help:      "Total assembled context token cost per coaching request, by feature.",
			Buckets:   []float64{128, 256, 512, 1024, 2048, 4096},
		},
		[]string{"feature"},
	)
)

func init() {
	prometheus.MustRegister(
		coachingSafetyBlockedTotal,
		coachingSafetyClassifyErrors,
		coachingPromptSectionTokens,
		coachingContextTokens,
	)
}
