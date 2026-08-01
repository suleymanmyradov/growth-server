package personalization

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/personalization"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toolStatusMessages maps tool names to user-friendly status messages
// shown as SSE "thinking" events while the tool executes.
var toolStatusMessages = map[string]string{
	"get_active_goals":         "Looking up your goals...",
	"get_active_habits":        "Looking up your habits...",
	"get_recent_check_ins":     "Reviewing your recent check-ins...",
	"get_latest_weekly_review": "Reviewing your weekly summary...",
	"get_pending_suggestions":  "Checking your plan suggestions...",
	"get_coaching_profile":     "Reviewing your coaching preferences...",
}

// historyEntry is a single prior conversation message (role + content),
// used to build the LLM message list. It's a local type to avoid pulling
// in a proto package just for a {role, content} pair.
type historyEntry struct {
	Role    string
	Content string
}

// streamAgenticCoaching is the agentic coaching path: it uses StreamAgent
// with on-demand tool calls so the model fetches user data (goals, habits,
// check-ins, etc.) only when the user's message makes it relevant.
//
// SSE event types (same as the legacy path, plus tool-status thinking):
//   - thinking:  {"message": "..."} — status updates while tools execute
//   - delta:     {"text": "..."} — incremental coaching text
//   - complete:  {"fullResponse": "..."} — final full response
//   - error:     {"message": "..."} — error before stream end
func streamAgenticCoaching(w http.ResponseWriter, r *http.Request, req *types.GeneratePersonalizedCoachingRequest, svcCtx *svc.ServiceContext, p principal.Principal) {
	streamStart := time.Now()
	ctx := r.Context()

	// --- Safety classification (same guardrail as the legacy ai-coach path) ---
	if svcCtx.Classifier != nil {
		classifyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		verdict, err := svcCtx.Classifier.Classify(classifyCtx, req.UserMessage)
		cancel()

		switch {
		case err != nil:
			logx.WithContext(ctx).Errorf("agentic coaching: safety classify failed, proceeding: user=%s err=%v", p.UserID, err)
		case verdict.Category == safety.CategoryCrisis || verdict.Category == safety.CategorySelfHarm:
			logx.WithContext(ctx).Infof("agentic coaching: safety block: user=%s category=%s", p.UserID, verdict.Category)
			streamCrisisResponse(w, ctx, req, svcCtx, p)
			return
		}
	}

	// --- Persist user message + fetch history (same as legacy path) ---
	history := fetchAndPersistHistory(ctx, req, svcCtx, p)

	// --- Fetch user profile for the system prompt (cheap, always useful) ---
	agenticCtx := personalization.AgenticCoachingContext{}
	if profileResp, err := svcCtx.AuthRpc.GetProfile(ctx, &authservice.GetProfileRequest{}); err != nil {
		logx.WithContext(ctx).Errorf("agentic coaching: failed to fetch user profile: %v", err)
	} else if profileResp.User != nil {
		agenticCtx.UserFullName = profileResp.User.FullName
		agenticCtx.UserBio = profileResp.User.Bio
		agenticCtx.UserLocation = profileResp.User.Location
	}

	// --- Build coaching tools (on-demand data retrieval) ---
	tools := personalization.BuildCoachingTools(p.UserID, personalization.CoachingToolDeps{
		Goals:           svcCtx.ClientRpc.Goals,
		Habits:          svcCtx.ClientRpc.Habits,
		CheckIns:        svcCtx.ClientRpc.CheckInService,
		WeeklyReviews:   svcCtx.ClientRpc.WeeklyReviewService,
		Personalization: svcCtx.ClientRpc.PersonalizationService,
	})

	// --- Build lean system prompt (no goals/habits/check-ins stuffed in) ---
	systemPrompt := personalization.BuildAgenticCoachingSystemPrompt(agenticCtx)

	// --- Build message list: prior history + current user message ---
	aiMessages := make([]ai.Message, 0, len(history)+1)
	for _, h := range history {
		role := ai.RoleUser
		if h.Role == "assistant" {
			role = ai.RoleAssistant
		}
		aiMessages = append(aiMessages, ai.Message{
			Role:    role,
			Content: h.Content,
		})
	}
	aiMessages = append(aiMessages, ai.Message{Role: ai.RoleUser, Content: req.UserMessage})

	// --- Open the agent stream ---
	logx.WithContext(ctx).Infof("agentic coaching: opening StreamAgent for user=%s", p.UserID)
	agentStream, err := svcCtx.AIClient.StreamAgent(ctx, ai.AgentRequest{
		ModelProfile:   ai.ModelChat,
		System:         systemPrompt,
		Messages:       aiMessages,
		Tools:          tools,
		MaxSteps:       6,
		MaxTotalTokens: 8000,
		Metadata: ai.Metadata{
			UserID:  p.UserID,
			Feature: "agentic_coaching_stream",
		},
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("agentic coaching: StreamAgent open failed: %v", err)
		errors.HandleGrpcError(w, status.Error(codes.Internal, "failed to start coaching stream"))
		return
	}
	defer agentStream.Close()

	// --- Commit SSE headers ---
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}

	// --- Thinking goroutine: sends periodic "thinking" SSE events while
	// the model processes before the first delta or tool event. This keeps
	// the connection alive and gives the user feedback during the initial
	// LLM call, which can take several seconds. ---
	firstEvent := make(chan struct{})
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		idx := 0
		for {
			select {
			case <-done:
				return
			case <-firstEvent:
				return
			case <-ticker.C:
				msg := coachingThinkingMessages[idx%len(coachingThinkingMessages)]
				idx++
				data, _ := json.Marshal(map[string]string{"message": msg})
				if _, err := fmt.Fprintf(w, "event: thinking\ndata: %s\n\n", data); err != nil {
					return
				}
				flush()
			}
		}
	}()
	// Ensure the thinking goroutine stops on all exit paths, including
	// early returns from fmt.Fprintf errors (client disconnect).
	defer close(done)

	signalFirstEvent := func() {
		select {
		case <-firstEvent:
		default:
			close(firstEvent)
		}
	}

	// --- Read agent stream and forward as SSE events ---
	deltaCount := 0

	for {
		chunk, recvErr := agentStream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				logx.WithContext(ctx).Errorf("agentic coaching: upstream EOF after %d deltas, %v elapsed", deltaCount, time.Since(streamStart))
				writeCoachingSSEError(w, flush, "stream ended before completion")
				return
			}
			logx.WithContext(ctx).Errorf("agentic coaching: stream recv error after %d deltas, %v elapsed: %v", deltaCount, time.Since(streamStart), recvErr)
			writeCoachingSSEError(w, flush, coachingGrpcErrMsg(recvErr))
			return
		}

		// Tool call event → send as "thinking" SSE event.
		if chunk.ToolCall != nil {
			signalFirstEvent()
			msg := toolStatusMessages[chunk.ToolCall.Name]
			if msg == "" {
				msg = fmt.Sprintf("Looking up information... (%s)", chunk.ToolCall.Name)
			}
			data, _ := json.Marshal(map[string]string{"message": msg})
			if _, err := fmt.Fprintf(w, "event: thinking\ndata: %s\n\n", data); err != nil {
				return
			}
			flush()
			continue
		}

		// Content delta → forward as "delta" SSE event.
		if chunk.Delta != "" {
			signalFirstEvent()
			deltaCount++
			deltaData, _ := json.Marshal(map[string]string{"text": chunk.Delta})
			if _, err := fmt.Fprintf(w, "event: delta\ndata: %s\n\n", deltaData); err != nil {
				return
			}
			flush()
			continue
		}

		// Complete event → send "complete" SSE event + persist assistant response.
		if chunk.Complete {
			data, _ := json.Marshal(map[string]string{"fullResponse": chunk.FullResponse})
			if _, err := fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data); err != nil {
				return
			}
			flush()

			if req.ConversationId != "" && chunk.FullResponse != "" {
				if _, err := svcCtx.AICoachRpc.ConversationService.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
					ConversationId: req.ConversationId,
					UserId:         p.UserID,
					Role:           "assistant",
					Content:        chunk.FullResponse,
				}); err != nil {
					logx.WithContext(ctx).Errorf("agentic coaching: failed to persist assistant message: %v", err)
				}
			}

			logx.WithContext(ctx).Infof("agentic coaching: complete after %d deltas, %v elapsed", deltaCount, time.Since(streamStart))
			return
		}
	}
}

// streamCrisisResponse sends the deterministic crisis response as SSE
// events and persists it as the assistant message.
func streamCrisisResponse(w http.ResponseWriter, ctx context.Context, req *types.GeneratePersonalizedCoachingRequest, svcCtx *svc.ServiceContext, p principal.Principal) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}

	deltaData, _ := json.Marshal(map[string]string{"text": safety.CrisisResponse})
	if _, err := fmt.Fprintf(w, "event: delta\ndata: %s\n\n", deltaData); err != nil {
		return
	}
	flush()

	completeData, _ := json.Marshal(map[string]string{"fullResponse": safety.CrisisResponse})
	if _, err := fmt.Fprintf(w, "event: complete\ndata: %s\n\n", completeData); err != nil {
		return
	}
	flush()

	if req.ConversationId != "" {
		if _, err := svcCtx.AICoachRpc.ConversationService.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
			ConversationId: req.ConversationId,
			UserId:         p.UserID,
			Role:           "assistant",
			Content:        safety.CrisisResponse,
		}); err != nil {
			logx.WithContext(ctx).Errorf("agentic coaching: failed to persist crisis response: %v", err)
		}
	}
}

// fetchAndPersistHistory persists the user message (if conversationId is
// provided) and fetches prior conversation history. Returns the history
// as a slice of historyEntry, excluding the just-appended user message.
func fetchAndPersistHistory(ctx context.Context, req *types.GeneratePersonalizedCoachingRequest, svcCtx *svc.ServiceContext, p principal.Principal) []historyEntry {
	var history []historyEntry
	if req.ConversationId == "" {
		return history
	}

	// Persist the user message.
	if _, err := svcCtx.AICoachRpc.ConversationService.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
		ConversationId: req.ConversationId,
		UserId:         p.UserID,
		Role:           "user",
		Content:        req.UserMessage,
	}); err != nil {
		logx.WithContext(ctx).Errorf("agentic coaching: failed to persist user message: %v", err)
	}

	// Fetch prior messages.
	msgsResp, err := svcCtx.AICoachRpc.ConversationService.GetMessages(ctx, &conversationservice.GetMessagesRequest{
		ConversationId: req.ConversationId,
		UserId:         p.UserID,
		Page:           1,
		Limit:          50,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("agentic coaching: failed to fetch history: %v", err)
		return history
	}

	msgs := msgsResp.Messages
	// Drop the last message if it's the user message we just appended.
	if n := len(msgs); n > 0 && msgs[n-1].Role == "user" && msgs[n-1].Content == req.UserMessage {
		msgs = msgs[:n-1]
	}
	for _, m := range msgs {
		history = append(history, historyEntry{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return history
}
