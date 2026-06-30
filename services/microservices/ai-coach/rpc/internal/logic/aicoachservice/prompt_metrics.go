package aicoachservicelogic

import (
	"context"
	"strconv"
	"strings"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"

	"github.com/zeromicro/go-zero/core/logx"
)

// buildCoachingUserPrompt assembles the personalized coaching user prompt
// within the token budget and records per-section + total-context metrics so
// the budget can be observed and tuned. feature labels the context-tokens
// histogram (e.g. "personalized_coaching_stream").
func buildCoachingUserPrompt(ctx context.Context, in prompts.PersonalizedCoachingInput, feature, userID string) string {
	userPrompt, breakdown := prompts.AssembleContextWithBreakdown(in, prompts.DefaultContextBudgetTokens)
	recordPromptBreakdown(ctx, breakdown, feature, userID)
	return userPrompt
}

func recordPromptBreakdown(ctx context.Context, breakdown []prompts.SectionBreakdown, feature, userID string) {
	total := 0
	var logBuf strings.Builder
	for _, s := range breakdown {
		included := "false"
		if s.Included {
			included = "true"
			total += s.Tokens
		}
		coachingPromptSectionTokens.WithLabelValues(s.Section, included).Observe(float64(s.Tokens))
		logBuf.WriteString(s.Section)
		logBuf.WriteString("=")
		logBuf.WriteString(strconv.Itoa(s.Tokens))
		if s.Included {
			logBuf.WriteString("(in)")
		} else {
			logBuf.WriteString("(dropped)")
		}
		logBuf.WriteString(" ")
	}
	coachingContextTokens.WithLabelValues(feature).Observe(float64(total))
	logx.WithContext(ctx).Infof("coaching prompt sections: user=%s feature=%s context_tokens=%d breakdown=[%s]",
		userID, feature, total, strings.TrimSpace(logBuf.String()))
}
