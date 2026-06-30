package aicoachservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
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
