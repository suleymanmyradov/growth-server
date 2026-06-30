package personalizationservicelogic

import (
	"context"
	"io"

	aiprompts "github.com/suleymanmyradov/growth-server/pkg/ai/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StreamPersonalizedCoachingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStreamPersonalizedCoachingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamPersonalizedCoachingLogic {
	return &StreamPersonalizedCoachingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StreamPersonalizedCoachingLogic) StreamPersonalizedCoaching(in *client.GeneratePersonalizedCoachingRequest, stream client.PersonalizationService_StreamPersonalizedCoachingServer) error {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "StreamPersonalizedCoachingLogic.StreamPersonalizedCoaching")
	defer span.End()

	if in.UserId == "" {
		return status.Error(codes.InvalidArgument, "userId is required")
	}

	// Get personalization context (same assembly as the unary version).
	contextReq := &client.GetPersonalizationContextRequest{
		UserId:       in.UserId,
		ForceRefresh: false,
	}
	contextLogic := NewGetPersonalizationContextLogic(ctx, l.svcCtx)
	contextResp, err := contextLogic.GetPersonalizationContext(contextReq)
	if err != nil {
		l.Errorf("failed to get personalization context: %v", err)
		return status.Error(codes.Internal, "failed to get personalization context")
	}

	// Build prompt input from the personalization context.
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

	// Calculate recent check-in summary.
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

	patternInsights := make(map[string]string, len(contextResp.Context.PatternInsights))
	for k, v := range contextResp.Context.PatternInsights {
		patternInsights[k] = v
	}

	// Build an aggregate check-in digest (counts + trend + top blocker) instead
	// of enumerating raw check-in rows. Trend/top-blocker come from the
	// pre-computed pattern insights so this stays near-constant size.
	recentCheckInsSummary := aiprompts.BuildContextSummary(
		len(contextResp.Context.RecentCheckIns),
		completionRate,
		patternInsights["top_blocker"],
		patternInsights["completion_pattern"],
	)

	// Convert history messages to the ai-coach proto type.
	history := make([]*aicoachservice.HistoryMessage, len(in.History))
	for i, h := range in.History {
		history[i] = &aicoachservice.HistoryMessage{
			Role:    h.Role,
			Content: h.Content,
		}
	}

	// Open the ai-coach streaming RPC and relay chunks to the client.
	aiStream, err := l.svcCtx.AICoachRpc.StreamPersonalizedCoaching(ctx, &aicoachservice.PersonalizedCoachingRequest{
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
		History:               history,
		UserFullName:          user.FullName,
		UserBio:               user.Bio,
		UserLocation:          user.Location,
		UserInterests:         user.Interests,
	})
	if err != nil {
		l.Errorf("AI coach stream open failed: %v", err)
		// Send a fallback response as a stream so the client still gets something.
		fallback := "I couldn't generate a coaching response right now, but pick one small action you can complete today and keep it easy. Small consistent actions build momentum over time."
		if sendErr := stream.Send(&client.PersonalizedCoachingStreamChunk{Delta: fallback}); sendErr != nil {
			return sendErr
		}
		return stream.Send(&client.PersonalizedCoachingStreamChunk{
			Complete:     true,
			FullResponse: fallback,
		})
	}

	for {
		chunk, recvErr := aiStream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				return nil
			}
			l.Errorf("AI coach stream recv error: %v", recvErr)
			return status.Errorf(codes.Internal, "AI stream interrupted: %v", recvErr)
		}

		if err := stream.Send(&client.PersonalizedCoachingStreamChunk{
			Delta:        chunk.Delta,
			Complete:     chunk.Complete,
			FullResponse: chunk.FullResponse,
		}); err != nil {
			l.Errorf("stream send error: %v", err)
			return err
		}

		if chunk.Complete {
			l.Infof("streaming personalized coaching complete: user=%s", in.UserId)
			return nil
		}
	}
}
