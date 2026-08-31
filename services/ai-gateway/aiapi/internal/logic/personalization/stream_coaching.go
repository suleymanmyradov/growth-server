package personalization

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/sse"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
)

// safetyCrisisThreshold is the minimum classifier confidence required to
// block the stream and send the deterministic crisis response. Low-confidence
// safety flags are logged and treated as safe to avoid false positives.
const safetyCrisisThreshold = 0.75

// toolStatusMessages maps tool names to user-friendly status messages
// shown as SSE "thinking" events while the tool executes.
var toolStatusMessages = map[string]string{
	"get_active_goals":         "Looking up your goals...",
	"get_active_habits":        "Looking up your habits...",
	"get_goal":                 "Getting goal details...",
	"get_habit":                "Getting habit details...",
	"get_recent_check_ins":     "Reviewing your recent check-ins...",
	"get_latest_weekly_review": "Reviewing your weekly summary...",
	"get_pending_suggestions":  "Checking your plan suggestions...",
	"get_coaching_profile":     "Reviewing your coaching preferences...",
	"propose_create_goal":      "Preparing a new goal for you to confirm...",
	"propose_update_goal":      "Preparing goal changes for you to confirm...",
	"propose_delete_goal":      "Preparing goal deletion for you to confirm...",
	"propose_create_habit":     "Preparing a new habit for you to confirm...",
	"propose_update_habit":     "Preparing habit changes for you to confirm...",
	"propose_delete_habit":     "Preparing habit deletion for you to confirm...",
	"search_articles":          "Searching articles...",
}

// coachingThinkingMessages are sent to the client as SSE "thinking" events
// while the model is processing before the first token arrives. Kept generic
// since this fires on every turn regardless of context — tool-specific
// status messages in toolStatusMessages handle the contextual cases.
var coachingThinkingMessages = []string{
	"Thinking...",
	"Composing a reply...",
	"Putting my thoughts together...",
	"Formulating my response...",
	"Gathering my thoughts...",
}

// ConversationStore is the subset of the ai-coach conversation service
// needed by the coaching stream. Defined here so the logic can be tested
// with a mock instead of the concrete gRPC client.
type ConversationStore interface {
	AppendMessage(ctx context.Context, in *conversationservice.AppendMessageRequest, opts ...grpc.CallOption) (*conversationservice.AppendMessageResponse, error)
	GetMessages(ctx context.Context, in *conversationservice.GetMessagesRequest, opts ...grpc.CallOption) (*conversationservice.GetMessagesResponse, error)
	RegenerateLastResponse(ctx context.Context, in *conversationservice.RegenerateLastResponseRequest, opts ...grpc.CallOption) (*conversationservice.RegenerateLastResponseResponse, error)
}

// ProfileFetcher is the subset of the auth service needed by the coaching
// stream.
type ProfileFetcher interface {
	GetProfile(ctx context.Context, in *authservice.GetProfileRequest, opts ...grpc.CallOption) (*authservice.GetProfileResponse, error)
}

// StreamCoachingDeps holds all external dependencies the coaching stream
// needs. Each field is an interface or a struct of interfaces so the
// orchestration can be tested with mocks.
type StreamCoachingDeps struct {
	AIClient       ai.Client
	Classifier     safety.Classifier
	Conversations  ConversationStore
	ProfileFetcher ProfileFetcher
	ToolDeps       CoachingToolDeps
	Config         config.CoachingConfig
}

// historyEntry is a single prior conversation message (role + content),
// used to build the LLM message list. It's a local type to avoid pulling
// in a proto package just for a {role, content} pair.
type historyEntry struct {
	Role    string
	Content string
}

// StreamCoaching is the agentic coaching orchestration: it uses StreamAgent
// with on-demand tool calls so the model fetches user data (goals, habits,
// check-ins, etc.) only when the user's message makes it relevant.
//
// SSE event types:
//   - reasoning: {"text": "..."} — model's live reasoning/thinking deltas (reasoning models only)
//   - thinking:  {"message": "..."} — status updates while tools execute
//   - delta:     {"text": "..."} — incremental coaching text
//   - proposal:  {id, action, payload} — a confirm/cancel card for a proposed CRUD action
//   - complete:  {"fullResponse": "..."} — final full response
//   - error:     {"message": "..."} — error before stream end
//
// The caller is responsible for request parsing, auth, and creating the
// *sse.Writer (so handlers that need to emit events before/after coaching —
// e.g. the voice-turn handler's transcript/audio events — can share the
// same writer). This function owns the streaming lifecycle from safety
// classification through response persistence.
//
// Returns the full coaching response text (empty on error or crisis
// short-circuit, in which case the appropriate SSE events have already
// been written).
func StreamCoaching(ctx context.Context, sseWriter *sse.Writer, req *types.GeneratePersonalizedCoachingRequest, p principal.Principal, deps StreamCoachingDeps) string {
	streamStart := time.Now()

	if req.GoalId != "" {
		if _, err := uuid.Parse(req.GoalId); err != nil {
			sseWriter.WriteEvent("error", map[string]string{"message": "Invalid goal context."})
			return ""
		}
	}

	if req.Regenerate {
		resp, err := deps.Conversations.RegenerateLastResponse(ctx, &conversationservice.RegenerateLastResponseRequest{
			ConversationId: req.ConversationId,
			UserId:         p.UserID,
		})
		if err != nil || resp.UserMessage == nil {
			logx.WithContext(ctx).Errorf("agentic coaching: failed to prepare response regeneration: %v", err)
			sseWriter.WriteEvent("error", map[string]string{"message": "Unable to regenerate this response. Please try again."})
			return ""
		}
		req.UserMessage = resp.UserMessage.Content
	}

	// --- Safety classification ---
	if deps.Classifier != nil {
		classifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		verdict, err := deps.Classifier.Classify(classifyCtx, req.UserMessage)
		cancel()

		switch {
		case err != nil:
			logx.WithContext(ctx).Errorf("agentic coaching: safety classify failed, proceeding: user=%s err=%v", p.UserID, err)
		case (verdict.Category == safety.CategoryCrisis || verdict.Category == safety.CategorySelfHarm) && verdict.Confidence >= safetyCrisisThreshold:
			logx.WithContext(ctx).Infof("agentic coaching: safety block: user=%s category=%s confidence=%.2f", p.UserID, verdict.Category, verdict.Confidence)
			streamCrisisResponse(ctx, sseWriter, req, p, deps.Conversations)
			return safety.CrisisResponse
		case verdict.Category == safety.CategoryCrisis || verdict.Category == safety.CategorySelfHarm:
			logx.WithContext(ctx).Infof("agentic coaching: safety flag below threshold, proceeding: user=%s category=%s confidence=%.2f reason=%q", p.UserID, verdict.Category, verdict.Confidence, verdict.Reason)
		}
	}

	// --- Persist user message + fetch history ---
	// Abort before the model call if the user turn cannot be stored: spending
	// tokens on an answer we know we cannot keep is the failure this guards.
	history, err := fetchAndPersistHistory(ctx, req, p, deps.Conversations, deps.Config)
	if err != nil {
		logx.WithContext(ctx).Errorf("agentic coaching: %v", err)
		sseWriter.WriteEvent("error", map[string]string{
			"message": "Could not save your message. Please try sending it again.",
		})
		return ""
	}

	// --- Fetch user profile for the system prompt (cheap, always useful) ---
	agenticCtx := AgenticCoachingContext{FocusGoalID: req.GoalId}
	if profileResp, err := deps.ProfileFetcher.GetProfile(ctx, &authservice.GetProfileRequest{}); err != nil {
		logx.WithContext(ctx).Errorf("agentic coaching: failed to fetch user profile: %v", err)
	} else if profileResp.User != nil {
		agenticCtx.UserFullName = profileResp.User.FullName
		agenticCtx.UserBio = profileResp.User.Bio
		agenticCtx.UserLocation = profileResp.User.Location
	}

	// --- Build coaching tools (on-demand data retrieval + proposals) ---
	tools := BuildCoachingTools(p.UserID, deps.ToolDeps)

	// --- Build lean system prompt (no goals/habits/check-ins stuffed in) ---
	systemPrompt := BuildAgenticCoachingSystemPrompt(agenticCtx)

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
	aiMessages = append(aiMessages, ai.Message{
		Role:        ai.RoleUser,
		Content:     req.UserMessage,
		Attachments: toAiAttachments(req.Attachments),
	})

	// --- Open the agent stream ---
	maxSteps, maxTotalTokens, maxTokens := coachingLimits(deps.Config)
	logx.WithContext(ctx).Infof("agentic coaching: opening StreamAgent for user=%s (maxSteps=%d, maxTotalTokens=%d, maxTokens=%d)", p.UserID, maxSteps, maxTotalTokens, maxTokens)
	agentStream, err := deps.AIClient.StreamAgent(ctx, ai.AgentRequest{
		ModelProfile:   ai.ModelChat,
		System:         systemPrompt,
		Messages:       aiMessages,
		Tools:          tools,
		MaxSteps:       maxSteps,
		MaxTokens:      &maxTokens,
		MaxTotalTokens: maxTotalTokens,
		Metadata: ai.Metadata{
			UserID:  p.UserID,
			Feature: "agentic_coaching_stream",
		},
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("agentic coaching: StreamAgent open failed: %v", err)
		sseWriter.WriteError("failed to start coaching stream")
		return ""
	}
	defer agentStream.Close()

	// --- Thinking goroutine ---
	thinking := sse.NewThinkingGuard(sseWriter, coachingThinkingMessages, 2*time.Second)
	thinking.Start()
	defer thinking.Stop()

	// --- Read agent stream and forward as SSE events ---
	deltaCount := 0
	var fullResponse string
	for {
		chunk, recvErr := agentStream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				logx.WithContext(ctx).Errorf("agentic coaching: upstream EOF after %d deltas, %v elapsed", deltaCount, time.Since(streamStart))
				sseWriter.WriteError("stream ended before completion")
				return ""
			}
			logx.WithContext(ctx).Errorf("agentic coaching: stream recv error after %d deltas, %v elapsed: %v", deltaCount, time.Since(streamStart), recvErr)
			sseWriter.WriteError(ai.UserFacingMessage(recvErr))
			return ""
		}

		if chunk.Reasoning != "" {
			thinking.SignalFirstEvent()
			if !sseWriter.WriteEvent("reasoning", map[string]string{"text": chunk.Reasoning}) {
				return ""
			}
			continue
		}

		if chunk.ToolCall != nil {
			thinking.SignalFirstEvent()

			switch chunk.ToolCall.Status {
			case ai.ToolStatusStarted:
				// Tool is about to execute — show a status message
				// immediately so the user sees activity while it runs.
				msg := toolStatusMessages[chunk.ToolCall.Name]
				if msg == "" {
					msg = fmt.Sprintf("Looking up information... (%s)", chunk.ToolCall.Name)
				}
				if !sseWriter.WriteEvent("thinking", map[string]string{"message": msg}) {
					return ""
				}

			case ai.ToolStatusCompleted:
				// Tool finished — emit proposal events for propose_* tools.
				if strings.HasPrefix(chunk.ToolCall.Name, "propose_") && chunk.ToolCall.Error == "" && chunk.ToolCall.Result != "" {
					proposalData, pErr := parseProposalResult(chunk.ToolCall.Result)
					if pErr == nil {
						if !sseWriter.WriteRaw("proposal", proposalData) {
							return ""
						}
					} else {
						logx.WithContext(ctx).Errorf("agentic coaching: failed to parse proposal result for %s: %v", chunk.ToolCall.Name, pErr)
					}
				}
			}

			continue
		}

		if chunk.Delta != "" {
			thinking.SignalFirstEvent()
			deltaCount++
			if !sseWriter.WriteEvent("delta", map[string]string{"text": chunk.Delta}) {
				return ""
			}
			continue
		}

		if chunk.Complete {
			fullResponse = chunk.FullResponse
			if req.ConversationId != "" && chunk.FullResponse != "" {
				if _, err := deps.Conversations.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
					ConversationId: req.ConversationId,
					UserId:         p.UserID,
					Role:           "assistant",
					Content:        chunk.FullResponse,
				}); err != nil {
					// Emit error, not complete. A `complete` event tells the
					// client the turn is durable; sending it after the write
					// failed makes the client cache a turn the server dropped,
					// which then vanishes on reload.
					logx.WithContext(ctx).Errorf("agentic coaching: failed to persist assistant message: %v", err)
					sseWriter.WriteEvent("error", map[string]string{
						"message": "Your coach replied, but the message could not be saved. Please try again.",
					})
					return ""
				}
			}

			if !sseWriter.WriteEvent("complete", map[string]string{"fullResponse": chunk.FullResponse}) {
				return ""
			}

			logx.WithContext(ctx).Infof("agentic coaching: complete after %d deltas, %v elapsed, finish_reason=%q", deltaCount, time.Since(streamStart), chunk.FinishReason)
			return fullResponse
		}
	}
}

// coachingLimits resolves the coaching config values with defaults.
func coachingLimits(cfg config.CoachingConfig) (maxSteps, maxTotalTokens, maxTokens int) {
	maxSteps = cfg.MaxSteps
	if maxSteps == 0 {
		maxSteps = 6
	}
	maxTotalTokens = cfg.MaxTotalTokens
	if maxTotalTokens == 0 {
		maxTotalTokens = 100000
	}
	maxTokens = cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	return
}

// historyLimits returns the model-context window: how many prior turns are
// replayed and the total character budget for them. Deliberately separate from
// the UI's history page size — the chat pane can page back through the entire
// conversation while the model sees only a bounded recent window.
func historyLimits(cfg config.CoachingConfig) (turns, maxChars int) {
	turns = cfg.HistoryTurns
	if turns <= 0 {
		turns = 20
	}
	maxChars = cfg.HistoryMaxChars
	if maxChars <= 0 {
		maxChars = 24000
	}
	return
}

// trimHistory keeps the most recent entries within the character budget,
// preserving chronological order. Trimming from the front (oldest first) keeps
// the turns nearest the current message, which are the ones that carry the
// thread of the conversation.
func trimHistory(history []historyEntry, maxChars int) []historyEntry {
	total := 0
	cut := 0
	for i := len(history) - 1; i >= 0; i-- {
		total += len(history[i].Content)
		if total > maxChars {
			cut = i + 1
			break
		}
	}
	if cut == 0 {
		return history
	}
	return history[cut:]
}

// streamCrisisResponse persists the user message, persists the deterministic
// crisis response, and then sends them as SSE events.
func streamCrisisResponse(ctx context.Context, sseWriter *sse.Writer, req *types.GeneratePersonalizedCoachingRequest, p principal.Principal, conversations ConversationStore) {
	// The crisis response is deterministic, so it is always delivered to the
	// user even if persistence fails — withholding a crisis message because a
	// database write failed would be the wrong trade. But the conversation
	// record must not silently lose the exchange either, so a failed write is
	// reported alongside the response rather than only logged.
	persisted := true
	if req.ConversationId != "" {
		if !req.Regenerate {
			if _, err := conversations.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
				ConversationId:  req.ConversationId,
				UserId:          p.UserID,
				Role:            "user",
				Content:         req.UserMessage,
				ClientMessageId: req.ClientMessageId,
			}); err != nil {
				logx.WithContext(ctx).Errorf("agentic coaching: failed to persist user message on crisis path: %v", err)
				persisted = false
			}
		}
		if _, err := conversations.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
			ConversationId: req.ConversationId,
			UserId:         p.UserID,
			Role:           "assistant",
			Content:        safety.CrisisResponse,
		}); err != nil {
			logx.WithContext(ctx).Errorf("agentic coaching: failed to persist crisis response: %v", err)
			persisted = false
		}
	}

	sseWriter.WriteEvent("delta", map[string]string{"text": safety.CrisisResponse})
	if !persisted {
		// Non-fatal: the response above stands. This tells the client the turn
		// will not survive a reload, so it does not present stale history as
		// complete.
		sseWriter.WriteEvent("error", map[string]string{
			"message": "This response could not be saved to your conversation history.",
		})
	}
	sseWriter.WriteEvent("complete", map[string]string{"fullResponse": safety.CrisisResponse})
}

// fetchAndPersistHistory fetches prior conversation history and then persists
// the user message. Fetching BEFORE appending avoids the fragile
// content-matching an earlier implementation used to exclude the just-appended
// message from the history list.
//
// A failure to persist the user message is returned, not logged and swallowed.
// Swallowing it produced the worst outcome available: the user receives a
// complete, well-formed coaching answer and the whole turn is gone on reload.
// The caller aborts the stream instead, so the user sees a retryable error
// before any tokens are spent.
//
// A history *fetch* failure is different and stays non-fatal: answering with
// less context is a degraded answer, not a lost turn.
func fetchAndPersistHistory(ctx context.Context, req *types.GeneratePersonalizedCoachingRequest, p principal.Principal, conversations ConversationStore, cfg config.CoachingConfig) ([]historyEntry, error) {
	var history []historyEntry
	if req.ConversationId == "" {
		return history, nil
	}

	turns, maxChars := historyLimits(cfg)

	// Fetch prior messages BEFORE appending the new user message.
	msgsResp, err := conversations.GetMessages(ctx, &conversationservice.GetMessagesRequest{
		ConversationId: req.ConversationId,
		UserId:         p.UserID,
		Page:           1,
		Limit:          int32(turns),
	})
	if err != nil {
		// Non-fatal: proceed with no history rather than failing the turn.
		logx.WithContext(ctx).Errorf("agentic coaching: failed to fetch history, proceeding without it: %v", err)
	} else {
		for _, m := range msgsResp.Messages {
			history = append(history, historyEntry{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	history = trimHistory(history, maxChars)

	if req.Regenerate {
		if len(history) > 0 && history[len(history)-1].Role == "user" {
			history = history[:len(history)-1]
		}
		return history, nil
	}

	// Persist the user message AFTER fetching history. Fatal on failure.
	if _, err := conversations.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
		ConversationId:  req.ConversationId,
		UserId:          p.UserID,
		Role:            "user",
		Content:         req.UserMessage,
		ClientMessageId: req.ClientMessageId,
	}); err != nil {
		return nil, fmt.Errorf("persist user message: %w", err)
	}

	return history, nil
}

// proposalPayload is the shape of a propose_* tool result, used to validate
// and re-marshal the result before forwarding it as an SSE "proposal" event.
type proposalPayload struct {
	Id      string         `json:"id"`
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload"`
}

// parseProposalResult validates the tool result JSON is a well-formed
// proposal ({id, action, payload}) and returns the re-marshaled JSON for
// the SSE event. Returns an error if the result is malformed or missing
// required fields.
func parseProposalResult(resultJSON string) (string, error) {
	var p proposalPayload
	if err := json.Unmarshal([]byte(resultJSON), &p); err != nil {
		return "", fmt.Errorf("unmarshal proposal result: %w", err)
	}
	if p.Id == "" || p.Action == "" {
		return "", fmt.Errorf("proposal result missing id or action")
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal proposal event: %w", err)
	}
	return string(b), nil
}

// toAiAttachments converts the request-level attachment list to the
// internal ai.Attachment shape. Unknown attachment types are dropped;
// empty data is also skipped because it cannot reach the model.
func toAiAttachments(in []types.Attachment) []ai.Attachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]ai.Attachment, 0, len(in))
	for _, a := range in {
		if a.Data == "" {
			continue
		}
		out = append(out, ai.Attachment{
			Type:        a.AttachmentType,
			Name:        a.Name,
			ContentType: a.ContentType,
			Data:        a.Data,
		})
	}
	return out
}
