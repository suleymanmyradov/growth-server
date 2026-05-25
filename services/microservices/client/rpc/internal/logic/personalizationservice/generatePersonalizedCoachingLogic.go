package personalizationservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (l *GeneratePersonalizedCoachingLogic) GeneratePersonalizedCoaching(in *client.GeneratePersonalizedCoachingRequest) (*client.GeneratePersonalizedCoachingResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get personalization context
	contextReq := &client.GetPersonalizationContextRequest{
		UserId:       in.UserId,
		ForceRefresh: false,
	}
	contextLogic := NewGetPersonalizationContextLogic(l.ctx, l.svcCtx)
	contextResp, err := contextLogic.GetPersonalizationContext(contextReq)
	if err != nil {
		l.Errorf("failed to get personalization context: %v", err)
		return nil, status.Error(codes.Internal, "failed to get personalization context")
	}

	// Build prompt input
	profile := contextResp.Context.Profile
	activeGoals := make([]string, len(contextResp.Context.ActiveGoals))
	for i, goal := range contextResp.Context.ActiveGoals {
		activeGoals[i] = goal.Title
	}

	activeHabits := make([]string, len(contextResp.Context.ActiveHabits))
	for i, habit := range contextResp.Context.ActiveHabits {
		activeHabits[i] = habit.Name
	}

	// Calculate recent check-in summary
	completedCount := 0
	for _, checkIn := range contextResp.Context.RecentCheckIns {
		if checkIn.Status == "completed" {
			completedCount++
		}
	}
	completionRate := 0.0
	if len(contextResp.Context.RecentCheckIns) > 0 {
		completionRate = float64(completedCount) / float64(len(contextResp.Context.RecentCheckIns)) * 100
	}

	checkInSummary := prompts.BuildContextSummary(
		len(contextResp.Context.RecentCheckIns),
		completionRate,
		"",
		"",
	)

	promptInput := prompts.PersonalizedCoachingInput{
		UserMessage:           in.UserMessage,
		AccountabilityStyle:   profile.AccountabilityStyle,
		PreferredTone:         profile.PreferredTone,
		DifficultyPreference:  profile.DifficultyPreference,
		ActiveGoals:           activeGoals,
		ActiveHabits:          activeHabits,
		RecentCheckInsSummary: checkInSummary,
		CommonBlockers:        profile.CommonBlockers,
		PatternInsights:       contextResp.Context.PatternInsights,
	}

	// Generate AI response
	aiResponse, aiErr := l.svcCtx.AIClient.Generate(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       prompts.BuildPersonalizedCoachingSystemPrompt(promptInput),
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: prompts.BuildPersonalizedCoachingUserPrompt(promptInput)}},
		Metadata:     ai.Metadata{UserID: userID.String(), Feature: "personalized_coaching"},
	})

	coachingResponse := ""
	if aiErr != nil {
		l.Errorf("AI generation failed: %v", aiErr)
		// Return a deterministic fallback response
		coachingResponse = "I couldn't generate a full coaching response right now, but based on your recent activity, pick one small action you can complete today and keep it easy. Small consistent actions build momentum over time."
	} else {
		coachingResponse = aiResponse.Message.Content
	}

	return &client.GeneratePersonalizedCoachingResponse{
		CoachingResponse: coachingResponse,
		Context:          contextResp.Context,
	}, nil
}
