package aicoachservicelogic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateWeeklyReviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateWeeklyReviewLogic {
	return &GenerateWeeklyReviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GenerateWeeklyReviewLogic) GenerateWeeklyReview(in *aicoach.WeeklyReviewRequest) (*aicoach.WeeklyReviewResponse, error) {
	if l.svcCtx.AIClient == nil {
		return &aicoach.WeeklyReviewResponse{
			AiSummary: "AI service is not configured. Please contact support.",
		}, nil
	}

	habitBreakdowns := make([]prompts.HabitBreakdownInput, len(in.HabitBreakdowns))
	for i, h := range in.HabitBreakdowns {
		habitBreakdowns[i] = prompts.HabitBreakdownInput{
			HabitID:        h.HabitId,
			HabitName:      h.HabitName,
			Category:       h.Category,
			CompletedCount: int(h.CompletedCount),
			MissedCount:    int(h.MissedCount),
			CompletionRate: float64(h.CompletionRate),
		}
	}

	blockerStats := make([]prompts.BlockerInput, len(in.BlockerStats))
	for i, b := range in.BlockerStats {
		blockerStats[i] = prompts.BlockerInput{
			Blocker: b.Blocker,
			Count:   int(b.Count),
		}
	}

	moodStats := make([]prompts.MoodInput, len(in.MoodStats))
	for i, m := range in.MoodStats {
		moodStats[i] = prompts.MoodInput{
			Mood:  m.Mood,
			Count: int(m.Count),
		}
	}

	energyStats := make([]prompts.EnergyInput, len(in.EnergyStats))
	for i, e := range in.EnergyStats {
		energyStats[i] = prompts.EnergyInput{
			Energy: e.Energy,
			Count:  int(e.Count),
		}
	}

	input := prompts.WeeklyReviewInput{
		AccountabilityStyle:  in.AccountabilityStyle,
		PreferredTone:        in.PreferredTone,
		DifficultyPreference: in.DifficultyPreference,
		CommonBlockers:       in.CommonBlockers,
		Goals:                in.Goals,
		TotalHabits:          int(in.TotalHabits),
		CompletionRate:       float64(in.CompletionRate),
		CompletedCheckIns:    int(in.CompletedCheckIns),
		MissedCheckIns:       int(in.MissedCheckIns),
		BestDay:              in.BestDay,
		HardestDay:           in.HardestDay,
		TopBlocker:           in.TopBlocker,
		HabitBreakdowns:      habitBreakdowns,
		BlockerStats:         blockerStats,
		MoodStats:            moodStats,
		EnergyStats:          energyStats,
		DetectedPatterns:     in.DetectedPatterns,
	}

	// Safety screen on all user-authored free text before it reaches the
	// model. Blockers, goal titles, and habit names are user-authored and
	// were previously interpolated unscreened. A flagged verdict or a
	// classifier failure (fail closed) falls back to the deterministic,
	// no-model review — the user still gets a summary, but unscreened or
	// unsafe text never reaches the LLM.
	if l.svcCtx.Classifier != nil {
		if freeText := weeklyReviewFreeText(in); freeText != "" {
			classifyCtx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
			verdict, err := safety.ClassifyWithRetry(classifyCtx, l.svcCtx.Classifier, freeText)
			cancel()
			switch {
			case err != nil:
				l.Errorf("weekly review safety classify failed, failing closed: user=%s err=%v", in.UserId, err)
				coachingSafetyClassifyErrors.Inc()
				return weeklyReviewFallbackResponse(input), nil
			case verdict.Category != safety.CategorySafe:
				if _, blocked := safety.BlockedResponse(verdict, safety.BlockConfidenceThreshold); !blocked {
					l.Infof("weekly review safety flag below threshold, proceeding: user=%s category=%s confidence=%.2f", in.UserId, verdict.Category, verdict.Confidence)
					break
				}
				// Reason deliberately not logged — may quote sensitive content.
				l.Infof("weekly review safety block: user=%s category=%s confidence=%.2f", in.UserId, verdict.Category, verdict.Confidence)
				coachingSafetyBlockedTotal.WithLabelValues(string(verdict.Category)).Inc()
				return weeklyReviewFallbackResponse(input), nil
			}
		}
	}

	systemPrompt := prompts.BuildWeeklyReviewSystemPrompt(in.AccountabilityStyle, in.PreferredTone, in.DifficultyPreference)
	userPrompt := prompts.BuildWeeklyReviewUserPrompt(input)

	resp, err := l.svcCtx.AIClient.Generate(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheapLong,
		System:       systemPrompt,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: userPrompt},
		},
		Metadata: ai.Metadata{
			UserID:  in.UserId,
			Feature: "weekly_review",
		},
	})
	if err != nil {
		l.Errorf("AI generate failed: %v", err)
		return &aicoach.WeeklyReviewResponse{
			AiSummary: "I couldn't generate your weekly review right now. Please try again later.",
		}, nil
	}

	// Parse the JSON response.
	var structured prompts.WeeklyReviewStructuredOutput
	if err := json.Unmarshal([]byte(resp.Message.Content), &structured); err != nil {
		l.Errorf("failed to parse weekly review JSON: %v, raw: %s", err, resp.Message.Content)
		// If JSON parsing fails, use the raw text as the summary.
		return &aicoach.WeeklyReviewResponse{
			AiSummary: resp.Message.Content,
		}, nil
	}

	return weeklyReviewResponseFromStructured(structured), nil
}

// weeklyReviewFreeText joins every user-authored string that lands in the
// weekly review prompt so it can be screened in a single classifier call.
func weeklyReviewFreeText(in *aicoach.WeeklyReviewRequest) string {
	var parts []string
	add := func(s string) {
		if t := strings.TrimSpace(s); t != "" {
			parts = append(parts, t)
		}
	}
	add(in.TopBlocker)
	for _, s := range in.Goals {
		add(s)
	}
	for _, s := range in.CommonBlockers {
		add(s)
	}
	for _, s := range in.DetectedPatterns {
		add(s)
	}
	for _, h := range in.HabitBreakdowns {
		add(h.HabitName)
		add(h.Category)
	}
	for _, b := range in.BlockerStats {
		add(b.Blocker)
	}
	for _, m := range in.MoodStats {
		add(m.Mood)
	}
	for _, e := range in.EnergyStats {
		add(e.Energy)
	}
	return strings.Join(parts, "\n")
}

// weeklyReviewFallbackResponse builds a deterministic review without calling
// the model — used when user text is flagged or unscreenable.
func weeklyReviewFallbackResponse(input prompts.WeeklyReviewInput) *aicoach.WeeklyReviewResponse {
	return weeklyReviewResponseFromStructured(prompts.GenerateDeterministicFallback(input))
}

func weeklyReviewResponseFromStructured(structured prompts.WeeklyReviewStructuredOutput) *aicoach.WeeklyReviewResponse {
	adjustments := make([]*aicoach.WeeklyReviewAdjustment, len(structured.SuggestedAdjustments))
	for i, a := range structured.SuggestedAdjustments {
		adjustments[i] = &aicoach.WeeklyReviewAdjustment{
			HabitId:        a.HabitID,
			HabitName:      a.HabitName,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
			AdjustmentType: a.AdjustmentType,
		}
	}

	return &aicoach.WeeklyReviewResponse{
		AiSummary:            structured.AiSummary,
		SuggestedAdjustments: adjustments,
		NextWeekPlan: &aicoach.NextWeekPlan{
			Focus:           structured.NextWeekPlan.Focus,
			Commitments:     structured.NextWeekPlan.Commitments,
			Risks:           structured.NextWeekPlan.Risks,
			RecoveryActions: structured.NextWeekPlan.RecoveryActions,
		},
	}
}
