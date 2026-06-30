package prompts

import "fmt"

// BuildContextSummary creates a concise, aggregate summary of recent check-ins
// for the AI. It is the single source of truth for the "check-in digest" so the
// prompt never enumerates raw check-in rows — only trends and counts.
//
// checkInCount is the number of recent check-ins considered; completionRate is
// the percentage (0-100) of those that were completed. topBlocker and
// recentTrend are optional; empty strings are omitted.
func BuildContextSummary(checkInCount int, completionRate float64, topBlocker, recentTrend string) string {
	var s string
	if recentTrend != "" {
		s = fmt.Sprintf("Recent activity: %d check-ins with %.1f%% completion rate. Trend: %s.", checkInCount, completionRate, recentTrend)
	} else {
		s = fmt.Sprintf("Recent activity: %d check-ins with %.1f%% completion rate.", checkInCount, completionRate)
	}
	if topBlocker != "" {
		s += fmt.Sprintf(" Most common blocker: %s.", topBlocker)
	}
	return s
}
