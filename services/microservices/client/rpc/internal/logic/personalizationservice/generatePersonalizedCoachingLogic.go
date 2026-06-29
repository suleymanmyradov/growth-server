package personalizationservicelogic

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GeneratePersonalizedCoachingLogic.GeneratePersonalizedCoaching")
	defer span.End()

	if in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}

	// Get personalization context
	contextReq := &client.GetPersonalizationContextRequest{
		UserId:       in.UserId,
		ForceRefresh: false,
	}
	contextLogic := NewGetPersonalizationContextLogic(ctx, l.svcCtx)
	contextResp, err := contextLogic.GetPersonalizationContext(contextReq)
	if err != nil {
		l.Errorf("failed to get personalization context: %v", err)
		return nil, status.Error(codes.Internal, "failed to get personalization context")
	}

	// Build prompt input
	profile := contextResp.Context.Profile
	user := contextResp.Context.User
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

	recentCheckInsSummary := fmt.Sprintf("Recent activity: %d check-ins with %.1f%% completion rate.",
		len(contextResp.Context.RecentCheckIns), completionRate)

	patternInsights := make(map[string]string, len(contextResp.Context.PatternInsights))
	for k, v := range contextResp.Context.PatternInsights {
		patternInsights[k] = v
	}

	aiResp, aiErr := l.svcCtx.AICoachRpc.GeneratePersonalizedCoaching(ctx, &aicoachservice.PersonalizedCoachingRequest{
		UserId:                in.UserId,
		UserMessage:           in.UserMessage,
		AccountabilityStyle:   profile.AccountabilityStyle,
		PreferredTone:         profile.PreferredTone,
		DifficultyPreference:  profile.DifficultyPreference,
		ActiveGoals:           activeGoals,
		ActiveHabits:          activeHabits,
		RecentCheckInsSummary: recentCheckInsSummary,
		CommonBlockers:        profile.CommonBlockers,
		PatternInsights:       patternInsights,
		UserFullName:          user.FullName,
		UserBio:               user.Bio,
		UserLocation:          user.Location,
		UserInterests:         user.Interests,
	})

	coachingResponse := ""
	if aiErr != nil {
		l.Errorf("AI generation failed: %v", aiErr)
		coachingResponse = "I couldn't generate a full coaching response right now, but based on your recent activity, pick one small action you can complete today and keep it easy. Small consistent actions build momentum over time."
	} else {
		coachingResponse = aiResp.CoachingResponse
	}

	return &client.GeneratePersonalizedCoachingResponse{
		CoachingResponse: coachingResponse,
		Context:          contextResp.Context,
	}, nil
}
