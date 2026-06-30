package aicoachservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
)

// onboardingHabitSuggestion is the JSON shape the model must return. It mirrors
// prompts.onboardingHabitsJSONShape.
type onboardingHabitSuggestion struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// onboardingFallbackHabits is returned when the AI client is unavailable or
// generation/parsing fails, so onboarding never blocks the user.
func onboardingFallbackHabits(goalTitle string, dailyMinutes int32) []*aicoach.OnboardingHabitSuggestion {
	per := int32(3)
	if dailyMinutes > 0 {
		per = dailyMinutes / 3
		if per < 1 {
			per = 1
		}
	}
	return []*aicoach.OnboardingHabitSuggestion{
		{Name: fmt.Sprintf("Work on %s for %d minutes", goalTitle, per), Description: "Set a timer and focus exclusively on this task."},
		{Name: "Review your plan for tomorrow", Description: "Spend 5 minutes each evening reviewing what you will do next."},
		{Name: "Track your progress", Description: "Write one sentence about what you accomplished today."},
	}
}

type GenerateOnboardingHabitsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateOnboardingHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateOnboardingHabitsLogic {
	return &GenerateOnboardingHabitsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GenerateOnboardingHabitsLogic) GenerateOnboardingHabits(in *aicoach.GenerateOnboardingHabitsRequest) (*aicoach.GenerateOnboardingHabitsResponse, error) {
	// Safety: classify the user-supplied free-text fields before they reach the
	// model. Crisis / self-harm short-circuit to the deterministic response so
	// the model never sees the input.
	if l.svcCtx.Classifier != nil {
		combined := strings.Join([]string{in.GoalTitle, in.Motivation, in.Blocker}, "\n")
		if combined != "" {
			classifyCtx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
			verdict, err := l.svcCtx.Classifier.Classify(classifyCtx, combined)
			cancel()
			switch {
			case err != nil:
				l.Errorf("onboarding safety classify failed, proceeding: user=%s err=%v", in.UserId, err)
				coachingSafetyClassifyErrors.Inc()
			case verdict.Category == safety.CategoryCrisis || verdict.Category == safety.CategorySelfHarm:
				l.Infof("onboarding safety block: user=%s category=%s confidence=%.2f reason=%s",
					in.UserId, verdict.Category, verdict.Confidence, verdict.Reason)
				coachingSafetyBlockedTotal.WithLabelValues(string(verdict.Category)).Inc()
				// Don't generate habits from crisis input; return the fallback
				// so onboarding can continue without surfacing the raw input.
				return &aicoach.GenerateOnboardingHabitsResponse{
					Habits: onboardingFallbackHabits(in.GoalTitle, in.DailyMinutes),
				}, nil
			}
		}
	}

	if l.svcCtx.AIClient == nil {
		return &aicoach.GenerateOnboardingHabitsResponse{
			Habits: onboardingFallbackHabits(in.GoalTitle, in.DailyMinutes),
		}, nil
	}

	systemPrompt := prompts.BuildOnboardingHabitsSystemPrompt(prompts.OnboardingHabitsInput{
		GoalTitle:           in.GoalTitle,
		GoalCategory:        in.GoalCategory,
		Motivation:          in.Motivation,
		Blocker:             in.Blocker,
		DailyMinutes:        in.DailyMinutes,
		AccountabilityStyle: in.AccountabilityStyle,
	})

	resp, err := l.svcCtx.AIClient.Generate(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelChat,
		System:       systemPrompt,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Generate 3 daily habits for my goal."},
		},
		ResponseFormat: ai.ResponseFormatJSON,
		Metadata: ai.Metadata{
			UserID:  in.UserId,
			Feature: "onboarding_habits",
		},
	})
	if err != nil {
		l.Errorf("onboarding habits AI generate failed: %v", err)
		return &aicoach.GenerateOnboardingHabitsResponse{
			Habits: onboardingFallbackHabits(in.GoalTitle, in.DailyMinutes),
		}, nil
	}

	habits, perr := parseOnboardingHabits(resp.Message.Content)
	if perr != nil {
		l.Errorf("onboarding habits parse failed, using fallback: %v", perr)
		return &aicoach.GenerateOnboardingHabitsResponse{
			Habits: onboardingFallbackHabits(in.GoalTitle, in.DailyMinutes),
		}, nil
	}

	return &aicoach.GenerateOnboardingHabitsResponse{
		Habits: habits,
	}, nil
}

// parseOnboardingHabits extracts and validates the JSON habit array from the
// model output, tolerating surrounding text and markdown fences. Returns at
// most 3 habits with non-empty names.
func parseOnboardingHabits(content string) ([]*aicoach.OnboardingHabitSuggestion, error) {
	c := strings.TrimSpace(content)
	// Strip markdown code fences if present.
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimPrefix(c, "```json")
		c = strings.TrimSuffix(c, "```")
		c = strings.TrimSpace(c)
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimPrefix(c, "```")
		c = strings.TrimSuffix(c, "```")
		c = strings.TrimSpace(c)
	}

	// Tolerate surrounding text by locating the JSON array bounds.
	start := strings.Index(c, "[")
	end := strings.LastIndex(c, "]")
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array found in model output")
	}
	c = c[start : end+1]

	var raw []onboardingHabitSuggestion
	if err := json.Unmarshal([]byte(c), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal habits: %w", err)
	}

	habits := make([]*aicoach.OnboardingHabitSuggestion, 0, 3)
	for _, h := range raw {
		if strings.TrimSpace(h.Name) == "" {
			continue
		}
		habits = append(habits, &aicoach.OnboardingHabitSuggestion{
			Name:        strings.TrimSpace(h.Name),
			Description: strings.TrimSpace(h.Description),
		})
		if len(habits) == 3 {
			break
		}
	}
	if len(habits) == 0 {
		return nil, fmt.Errorf("no valid habits parsed")
	}
	return habits, nil
}
