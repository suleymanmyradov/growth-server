package aicoachservicelogic

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
)

type GeneratePersonalizedCoachingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGeneratePersonalizedCoachingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GeneratePersonalizedCoachingLogic {
	return &GeneratePersonalizedCoachingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GeneratePersonalizedCoachingLogic) GeneratePersonalizedCoaching(in *aicoach.PersonalizedCoachingRequest) (*aicoach.PersonalizedCoachingResponse, error) {
	if in.UserMessage == "" {
		return &aicoach.PersonalizedCoachingResponse{
			CoachingResponse: "I didn't catch that — could you say a bit more about what you'd like help with?",
		}, nil
	}

	// Hard safety guardrail: classify the fresh user message before calling
	// the model. Crisis / self-harm never reach the model.
	if l.svcCtx.Classifier != nil {
		classifyCtx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
		verdict, err := safety.ClassifyWithRetry(classifyCtx, l.svcCtx.Classifier, in.UserMessage)
		cancel()

		switch {
		case err != nil:
			// Fail closed: unscreened input never reaches the model. The
			// deterministic degraded-mode response still carries crisis
			// resources for the user who may need them.
			l.Errorf("coaching safety classify failed, failing closed: user=%s err=%v", in.UserId, err)
			coachingSafetyClassifyErrors.Inc()
			return &aicoach.PersonalizedCoachingResponse{
				CoachingResponse: safety.UnavailableResponse,
			}, nil

		case verdict.Category == safety.CategoryCrisis || verdict.Category == safety.CategorySelfHarm:
			// Reason deliberately not logged — classifier reasons can quote
			// self-harm content verbatim.
			l.Infof("coaching safety block: user=%s category=%s confidence=%.2f",
				in.UserId, verdict.Category, verdict.Confidence)
			coachingSafetyBlockedTotal.WithLabelValues(string(verdict.Category)).Inc()
			return &aicoach.PersonalizedCoachingResponse{
				CoachingResponse: prompts.CrisisResponse,
			}, nil
		}
	}

	if l.svcCtx.AIClient == nil {
		return &aicoach.PersonalizedCoachingResponse{
			CoachingResponse: "I'm not fully configured right now, but here's my advice: pick one small action you can complete today. Small consistent steps build momentum.",
		}, nil
	}

	input := prompts.PersonalizedCoachingInput{
		UserMessage:           in.UserMessage,
		AccountabilityStyle:   in.AccountabilityStyle,
		PreferredTone:         in.PreferredTone,
		DifficultyPreference:  in.DifficultyPreference,
		ActiveGoals:           in.ActiveGoals,
		ActiveHabits:          in.ActiveHabits,
		RecentCheckInsSummary: in.RecentCheckInsSummary,
		CommonBlockers:        in.CommonBlockers,
		PatternInsights:       in.PatternInsights,
		UserFullName:          in.UserFullName,
		UserBio:               in.UserBio,
		UserLocation:          in.UserLocation,
		UserInterests:         in.UserInterests,
	}

	systemPrompt := prompts.BuildPersonalizedCoachingSystemPrompt(input)
	userPrompt := buildCoachingUserPrompt(l.ctx, input, "personalized_coaching", in.UserId)

	resp, err := l.svcCtx.AIClient.Generate(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelChat,
		System:       systemPrompt,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: userPrompt},
		},
		Metadata: ai.Metadata{
			UserID:  in.UserId,
			Feature: "personalized_coaching",
		},
	})
	if err != nil {
		l.Errorf("AI generate failed: %v", err)
		return &aicoach.PersonalizedCoachingResponse{
			CoachingResponse: "I couldn't generate a coaching response right now, but pick one small action you can complete today and keep it easy. Small consistent actions build momentum over time.",
		}, nil
	}

	return &aicoach.PersonalizedCoachingResponse{
		CoachingResponse: resp.Message.Content,
	}, nil
}
