package personalization

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// thinkingMessages are sent to the client as SSE "thinking" events while the
// model is processing before the first token arrives.
var coachingThinkingMessages = []string{
	"Reviewing your goals and habits...",
	"Looking at your recent check-ins...",
	"Considering your patterns and blockers...",
	"Reflecting on your coaching preferences...",
	"Crafting your personalized response...",
}

// StreamPersonalizedCoachingHandler is an SSE endpoint that streams the
// AI-generated personalized coaching response as it is produced.
//
// SSE event types:
//   - thinking:  {"message": "..."} — status updates while the model processes
//   - delta:     {"text": "..."} — incremental coaching text
//   - complete:  {"fullResponse": "..."} — final full response
//   - error:     {"message": "..."} — error before stream end
func StreamPersonalizedCoachingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GeneratePersonalizedCoachingRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		p, ok := principal.PrincipalFrom(r.Context())
		if !ok {
			errors.HandleGrpcError(w, status.Error(codes.Unauthenticated, "not authenticated"))
			return
		}

		// If a conversationId is provided, persist the user message before
		// opening the stream so the message is saved even if streaming fails.
		// Also fetch prior conversation history to give the LLM context.
		var history []*clientpersonalization.HistoryMessage
		if req.ConversationId != "" {
			if _, err := svcCtx.ConversationRpc.AppendMessage(r.Context(), &conversationservice.AppendMessageRequest{
				ConversationId: req.ConversationId,
				UserId:         p.UserID,
				Role:           "user",
				Content:        req.UserMessage,
			}); err != nil {
				logx.WithContext(r.Context()).Errorf("SSE coaching stream: failed to persist user message: %v", err)
				// Non-fatal: continue streaming even if persistence fails.
			}

			// Fetch prior messages (before the one we just appended) so the
			// LLM can see the conversation context. We exclude the latest
			// user message since it's already passed as UserMessage.
			if msgsResp, err := svcCtx.ConversationRpc.GetMessages(r.Context(), &conversationservice.GetMessagesRequest{
				ConversationId: req.ConversationId,
				UserId:         p.UserID,
				Page:           1,
				Limit:          50,
			}); err != nil {
				logx.WithContext(r.Context()).Errorf("SSE coaching stream: failed to fetch history: %v", err)
				// Non-fatal: continue without history.
			} else {
				// Drop the last message if it's the user message we just
				// appended (it equals req.UserMessage and has role "user").
				msgs := msgsResp.Messages
				if n := len(msgs); n > 0 && msgs[n-1].Role == "user" && msgs[n-1].Content == req.UserMessage {
					msgs = msgs[:n-1]
				}
				for _, m := range msgs {
					history = append(history, &clientpersonalization.HistoryMessage{
						Role:    m.Role,
						Content: m.Content,
					})
				}
			}
		}

		// Open the client-rpc streaming RPC before writing any response, so a
		// dial-time error can be surfaced as a proper HTTP status code rather
		// than an in-band SSE error event after a committed 200.
		streamStart := time.Now()
		logx.WithContext(r.Context()).Infof("SSE coaching stream: opening client-rpc stream for user=%s", p.UserID)
		stream, err := svcCtx.PersonalizationRpc.StreamPersonalizedCoaching(r.Context(), &clientpersonalization.GeneratePersonalizedCoachingRequest{
			UserId:         p.UserID,
			UserMessage:    req.UserMessage,
			Context:        req.Context,
			ConversationId: req.ConversationId,
			History:        history,
		})
		if err != nil {
			logx.WithContext(r.Context()).Errorf("SSE coaching stream: client-rpc stream open failed after %v: %v", time.Since(streamStart), err)
			errors.HandleGrpcError(w, err)
			return
		}
		logx.WithContext(r.Context()).Infof("SSE coaching stream: client-rpc stream opened after %v", time.Since(streamStart))

		// Set SSE headers and commit a 200 now that the upstream stream is open.
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

		// Start a thinking goroutine that sends periodic "thinking" SSE events
		// while the model processes the prompt. Stops on first delta or stream end.
		firstDelta := make(chan struct{})
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			idx := 0
			for {
				select {
				case <-done:
					return
				case <-firstDelta:
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

		deltaCount := 0
		var totalDeltaChars int
		firstDeltaSent := false
		for {
			chunk, recvErr := stream.Recv()
			if recvErr != nil {
				close(done)
				if recvErr == io.EOF {
					logx.WithContext(r.Context()).Errorf("SSE coaching stream: upstream EOF after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
					writeCoachingSSEError(w, flush, "stream ended before completion")
					return
				}
				logx.WithContext(r.Context()).Errorf("SSE coaching stream: upstream recv error after %d deltas, %d chars, %v elapsed: %v", deltaCount, totalDeltaChars, time.Since(streamStart), recvErr)
				writeCoachingSSEError(w, flush, coachingGrpcErrMsg(recvErr))
				return
			}

			if chunk.Complete {
				close(done)
				data, _ := json.Marshal(map[string]string{"fullResponse": chunk.FullResponse})
				if _, err := fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data); err != nil {
					return
				}
				flush()

				// Persist the assistant response if a conversationId was provided.
				if req.ConversationId != "" && chunk.FullResponse != "" {
					if _, err := svcCtx.ConversationRpc.AppendMessage(r.Context(), &conversationservice.AppendMessageRequest{
						ConversationId: req.ConversationId,
						UserId:         p.UserID,
						Role:           "assistant",
						Content:        chunk.FullResponse,
					}); err != nil {
						logx.WithContext(r.Context()).Errorf("SSE coaching stream: failed to persist assistant message: %v", err)
					}
				}

				logx.WithContext(r.Context()).Infof("SSE coaching stream: complete event sent after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
				return
			}

			// Empty delta is a heartbeat keepalive.
			if chunk.Delta == "" {
				if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
					return
				}
				flush()
				continue
			}

			// Signal the thinking goroutine to stop on the first real delta.
			if !firstDeltaSent {
				firstDeltaSent = true
				close(firstDelta)
			}
			deltaCount++
			totalDeltaChars += len(chunk.Delta)
			deltaData, _ := json.Marshal(map[string]string{"text": chunk.Delta})
			if _, err := fmt.Fprintf(w, "event: delta\ndata: %s\n\n", deltaData); err != nil {
				logx.WithContext(r.Context()).Errorf("SSE coaching stream: write delta error after %d deltas: %v", deltaCount, err)
				return
			}
			flush()
		}
	}
}

func writeCoachingSSEError(w http.ResponseWriter, flush func(), msg string) {
	data, _ := json.Marshal(map[string]string{"message": msg})
	if _, err := fmt.Fprintf(w, "event: error\ndata: %s\n\n", data); err != nil {
		return
	}
	flush()
}

func coachingGrpcErrMsg(err error) string {
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	return err.Error()
}
