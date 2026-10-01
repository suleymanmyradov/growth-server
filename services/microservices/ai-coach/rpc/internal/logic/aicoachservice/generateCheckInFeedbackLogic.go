package aicoachservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateCheckInFeedbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateCheckInFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateCheckInFeedbackLogic {
	return &GenerateCheckInFeedbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GenerateCheckInFeedbackLogic) GenerateCheckInFeedback(in *aicoach.CheckInFeedbackRequest) (*aicoach.CheckInFeedbackResponse, error) {
	if l.svcCtx.AIClient == nil {
		return &aicoach.CheckInFeedbackResponse{
			Feedback: "Keep going! Every check-in builds momentum.",
		}, nil
	}

	// Hard safety guardrail: classify the free-text check-in fields before
	// they reach the model. Blocker and Note are user-authored and were
	// previously passed through unscreened (crisis, self-harm, and medical
	// content all reached the prompt verbatim).
	if l.svcCtx.Classifier != nil {
		freeText := checkInFreeText(in)
		if freeText != "" {
			classifyCtx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
			verdict, err := safety.ClassifyWithRetry(classifyCtx, l.svcCtx.Classifier, freeText)
			cancel()

			switch {
			case err != nil:
				// Fail closed: unscreened input never reaches the model.
				l.Errorf("check-in safety classify failed, failing closed: user=%s err=%v", in.UserId, err)
				coachingSafetyClassifyErrors.Inc()
				return &aicoach.CheckInFeedbackResponse{
					Feedback: "Keep showing up — every check-in counts.",
				}, nil
			case verdict.Category != safety.CategorySafe:
				resp, blocked := safety.BlockedResponse(verdict, safety.BlockConfidenceThreshold)
				if !blocked {
					l.Infof("check-in safety flag below threshold, proceeding: user=%s category=%s confidence=%.2f",
						in.UserId, verdict.Category, verdict.Confidence)
					break
				}
				// Reason deliberately not logged — classifier reasons can
				// quote sensitive content verbatim.
				l.Infof("check-in safety block: user=%s category=%s confidence=%.2f",
					in.UserId, verdict.Category, verdict.Confidence)
				coachingSafetyBlockedTotal.WithLabelValues(string(verdict.Category)).Inc()
				return &aicoach.CheckInFeedbackResponse{
					Feedback: resp,
				}, nil
			}
		}
	}

	input := prompts.CheckInFeedbackInput{
		HabitName:            in.HabitName,
		Status:               in.Status,
		Mood:                 in.Mood,
		Energy:               in.Energy,
		Blocker:              in.Blocker,
		Note:                 in.Note,
		AccountabilityStyle:  in.AccountabilityStyle,
		PreferredTone:        in.PreferredTone,
		DifficultyPreference: in.DifficultyPreference,
		CommonBlockers:       in.CommonBlockers,
		Streak:               in.Streak,
		RecentPattern:        in.RecentPattern,
	}

	systemPrompt := prompts.BuildSystemPrompt(in.AccountabilityStyle, in.PreferredTone, in.DifficultyPreference)
	userPrompt := prompts.BuildUserPrompt(input)

	resp, err := l.svcCtx.AIClient.Generate(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       systemPrompt,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: userPrompt},
		},
		Metadata: ai.Metadata{
			UserID:  in.UserId,
			Feature: "checkin_feedback",
		},
	})
	if err != nil {
		l.Errorf("AI generate failed: %v", err)
		return &aicoach.CheckInFeedbackResponse{
			Feedback: "Keep showing up — every check-in counts.",
		}, nil
	}

	return &aicoach.CheckInFeedbackResponse{
		Feedback: resp.Message.Content,
	}, nil
}

// checkInFreeText joins every user-authored free-text field that gets
// interpolated into the feedback prompt so they can be screened together in
// a single classifier call.
func checkInFreeText(in *aicoach.CheckInFeedbackRequest) string {
	parts := make([]string, 0, 7+len(in.CommonBlockers))
	for _, s := range []string{
		in.HabitName, in.Blocker, in.Note, in.RecentPattern,
		in.Status, in.Mood, in.Energy,
	} {
		if t := strings.TrimSpace(s); t != "" {
			parts = append(parts, t)
		}
	}
	for _, s := range in.CommonBlockers {
		if t := strings.TrimSpace(s); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
}
