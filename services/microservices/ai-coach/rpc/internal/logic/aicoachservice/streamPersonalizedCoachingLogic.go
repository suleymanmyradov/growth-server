package aicoachservicelogic

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *StreamPersonalizedCoachingLogic) StreamPersonalizedCoaching(in *aicoach.PersonalizedCoachingRequest, stream aicoach.AICoachService_StreamPersonalizedCoachingServer) error {
	if in.UserMessage == "" {
		return stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
			Complete:     true,
			FullResponse: "I didn't catch that — could you say a bit more about what you'd like help with?",
		})
	}

	// Hard safety guardrail: classify the fresh user message before building
	// the prompt or calling the model. Crisis / self-harm never reach the
	// model — a pre-written response can't hallucinate or give harmful advice.
	// The system-prompt line ("If the user expresses crisis...") remains as
	// defense-in-depth, but the classifier is the primary control.
	if l.svcCtx.Classifier != nil {
		classifyCtx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
		verdict, err := l.svcCtx.Classifier.Classify(classifyCtx, in.UserMessage)
		cancel()

		switch {
		case err != nil:
			// Fail-open for availability (matches the classifier's own
			// "unparseable -> safe" default), but log + metric so outages
			// are visible.
			l.Errorf("coaching safety classify failed, proceeding: user=%s err=%v", in.UserId, err)
			coachingSafetyClassifyErrors.Inc()

		case verdict.Category == safety.CategoryCrisis || verdict.Category == safety.CategorySelfHarm:
			l.Infof("coaching safety block: user=%s category=%s confidence=%.2f reason=%s",
				in.UserId, verdict.Category, verdict.Confidence, verdict.Reason)
			coachingSafetyBlockedTotal.WithLabelValues(string(verdict.Category)).Inc()
			// Stream the deterministic response over the same SSE channel so
			// the frontend renders it like a normal assistant message. The
			// gateway persists FullResponse on the complete event, so history
			// stays consistent without a separate persistence call here.
			if err := stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
				Delta: prompts.CrisisResponse,
			}); err != nil {
				return err
			}
			return stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
				Complete:     true,
				FullResponse: prompts.CrisisResponse,
			})
		}
	}

	// If the AI client is not configured, send a fallback response.
	if l.svcCtx.AIClient == nil {
		fallback := "I'm not fully configured right now, but here's my advice: pick one small action you can complete today. Small consistent steps build momentum."
		if err := stream.Send(&aicoach.PersonalizedCoachingStreamChunk{Delta: fallback}); err != nil {
			return err
		}
		return stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
			Complete:     true,
			FullResponse: fallback,
		})
	}

	// Long-term memory retrieval (Workstream 2). Fetches cross-conversation
	// snippets from the private user_memory index and injects them at the
	// LOWEST priority tier of the context budget (see context_assembler.go),
	// so retrieval enhances the prompt without ever blowing the Stage-1 token
	// budget. Fail-open: errors/timeouts log + metric and yield no snippets.
	memories := l.retrieveMemories(in.UserId, in.UserMessage, in.History, in.RecentCheckInsSummary)

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
		RelevantMemories:      memories,
	}

	systemPrompt := prompts.BuildPersonalizedCoachingSystemPrompt(input)
	userPrompt := buildCoachingUserPrompt(l.ctx, input, "personalized_coaching_stream", in.UserId)

	// Build the message list: prior conversation history (if any) followed by
	// the current user prompt. This gives the LLM context from earlier turns.
	aiMessages := make([]ai.Message, 0, len(in.History)+1)
	for _, h := range in.History {
		role := ai.RoleUser
		if h.Role == "assistant" {
			role = ai.RoleAssistant
		}
		aiMessages = append(aiMessages, ai.Message{
			Role:    role,
			Content: h.Content,
		})
	}
	// The current turn's user message (with personalization context appended).
	aiMessages = append(aiMessages, ai.Message{Role: ai.RoleUser, Content: userPrompt})

	aiStream, err := l.svcCtx.AIClient.Stream(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelChat,
		System:       systemPrompt,
		Messages:     aiMessages,
		Metadata: ai.Metadata{
			UserID:  in.UserId,
			Feature: "personalized_coaching_stream",
		},
	})
	if err != nil {
		l.Errorf("AI stream open failed: %v", err)
		fallback := "I couldn't generate a coaching response right now, but pick one small action you can complete today and keep it easy. Small consistent actions build momentum over time."
		if sendErr := stream.Send(&aicoach.PersonalizedCoachingStreamChunk{Delta: fallback}); sendErr != nil {
			return sendErr
		}
		return stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
			Complete:     true,
			FullResponse: fallback,
		})
	}
	defer aiStream.Close()

	var fullResponse strings.Builder
	for {
		chunk, recvErr := aiStream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}
			l.Errorf("AI stream recv error: %v", recvErr)
			// Send what we have so far as complete.
			if fullResponse.Len() > 0 {
				return stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
					Complete:     true,
					FullResponse: fullResponse.String(),
				})
			}
			// Nothing received — send fallback.
			fallback := "I had trouble generating a response. Please try again."
			_ = stream.Send(&aicoach.PersonalizedCoachingStreamChunk{Delta: fallback})
			return stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
				Complete:     true,
				FullResponse: fallback,
			})
		}

		if chunk.Delta != "" {
			fullResponse.WriteString(chunk.Delta)
			if err := stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
				Delta: chunk.Delta,
			}); err != nil {
				return err
			}
		}
	}

	l.Infof("streaming personalized coaching complete: user=%s, %d chars", in.UserId, fullResponse.Len())
	return stream.Send(&aicoach.PersonalizedCoachingStreamChunk{
		Complete:     true,
		FullResponse: fullResponse.String(),
	})
}
