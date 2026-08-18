package voice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/logic/personalization"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	clientpb "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SSE event types for POST /personalization/voice-turn:
//   transcript  {"text":"...","language":"...","duration":...}  — STT result
//   delta       {"text":"..."}                                  — coaching text delta
//   complete    {"fullResponse":"..."}                          — final coaching text
//   audio       {"format":"mp3","data":"<base64>"}              — synthesized TTS audio
//   conversation{"id":"..."}                                    — conversation created/used
//   error       {"message":"..."}                               — non-fatal error
//   ready       {}                                              — ready for the next turn

// maxVoiceTurnBytes caps a single voice utterance upload (25 MB, matching the
// OpenAI Whisper limit).
const maxVoiceTurnBytes = 25 << 20

// VoiceTurnHandler accepts a multipart audio upload (field "audio") plus
// optional "language" and "conversationId" form fields, and responds with an
// SSE stream that carries the transcription, the streamed coaching response,
// and the synthesized spoken audio. This is the live voice chat transport —
// turn-based: the client records one utterance, uploads it, and plays back the
// spoken response. It reuses the existing BFF proxy (SSE streams through).
func VoiceTurnHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principal.PrincipalFrom(r.Context())
		if !ok {
			errors.HandleGrpcError(w, status.Error(codes.Unauthenticated, "not authenticated"))
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxVoiceTurnBytes)
		if err := r.ParseMultipartForm(maxVoiceTurnBytes); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		file, header, err := r.FormFile("audio")
		if err != nil {
			errors.WriteParseError(w, err)
			return
		}
		defer file.Close()

		format := audioFormatFromHeader(header.Filename, header.Header.Get("Content-Type"))
		if format == "" {
			errors.WriteParseError(w, status.Error(codes.InvalidArgument, "could not determine audio format from filename or content-type"))
			return
		}

		audio, err := io.ReadAll(file)
		if err != nil {
			errors.WriteParseError(w, err)
			return
		}
		if len(audio) == 0 {
			errors.WriteParseError(w, status.Error(codes.InvalidArgument, "audio payload is empty"))
			return
		}

		language := r.FormValue("language")
		conversationID := r.FormValue("conversationId")

		// Commit the SSE response now; everything else is streamed.
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
		writeEvent := func(event string, payload any) {
			data, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			flush()
		}
		writeError := func(msg string) {
			writeEvent("error", map[string]string{"message": msg})
		}

		ctx := r.Context()
		start := time.Now()

		// 1. Transcribe the utterance.
		transcribeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		transResp, err := svcCtx.AICoachRpc.AICoachService.Transcribe(transcribeCtx, &aicoachservice.TranscribeRequest{
			UserId:   p.UserID,
			Audio:    audio,
			Format:   format,
			Language: language,
		})
		cancel()
		if err != nil {
			logx.WithContext(ctx).Errorf("voice turn: transcribe failed after %v: %v", time.Since(start), err)
			writeError("I couldn't understand your audio. Please try speaking again.")
			return
		}
		userText := transResp.Text
		if userText == "" {
			writeError("transcription was empty")
			return
		}
		writeEvent("transcript", map[string]any{
			"text":     userText,
			"language": transResp.Language,
			"duration": transResp.Duration,
		})

		// 2. Ensure a conversation exists and persist the user message.
		if conversationID == "" {
			title := userText
			if len(title) > 60 {
				title = title[:60] + "…"
			}
			convResp, err := svcCtx.AICoachRpc.ConversationService.StartConversation(ctx, &conversationservice.StartConversationRequest{
				UserId: p.UserID,
				Type:   "coach",
				Title:  title,
			})
			if err != nil {
				logx.WithContext(ctx).Errorf("voice turn: start conversation: %v", err)
			} else if convResp.Conversation != nil {
				conversationID = convResp.Conversation.Id
				writeEvent("conversation", map[string]string{"id": conversationID})
			}
		} else {
			if _, err := svcCtx.AICoachRpc.ConversationService.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
				ConversationId: conversationID,
				UserId:         p.UserID,
				Role:           "user",
				Content:        userText,
			}); err != nil {
				logx.WithContext(ctx).Errorf("voice turn: persist user message: %v", err)
			}
		}

		// 3. Fetch conversation history for LLM context.
		history := fetchVoiceHistory(ctx, svcCtx, conversationID, p.UserID)

		// 4. Fetch personalization context.
		contextResp, err := svcCtx.ClientRpc.PersonalizationService.GetPersonalizationContext(ctx, &clientpersonalization.GetPersonalizationContextRequest{
			UserId:       p.UserID,
			ForceRefresh: false,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("voice turn: personalization context: %v", err)
			writeError(ai.UserFacingMessage(err))
			return
		}

		// 5. Open the ai-coach streaming RPC.
		aiReq := personalization.BuildPersonalizedCoachingRequest(p.UserID, userText, history, contextResp.Context)
		stream, err := svcCtx.AICoachRpc.AICoachService.StreamPersonalizedCoaching(ctx, aiReq)
		if err != nil {
			logx.WithContext(ctx).Errorf("voice turn: coaching stream open: %v", err)
			writeError(ai.UserFacingMessage(err))
			return
		}

		// 6. Forward deltas; collect the full response.
		var fullResponse string
		for {
			chunk, recvErr := stream.Recv()
			if recvErr != nil {
				if fullResponse != "" {
					break
				}
				logx.WithContext(ctx).Errorf("voice turn: coaching stream recv: %v", recvErr)
				writeError(ai.UserFacingMessage(recvErr))
				return
			}
			if chunk.Complete {
				fullResponse = chunk.FullResponse
				break
			}
			if chunk.Delta != "" {
				fullResponse += chunk.Delta
				writeEvent("delta", map[string]string{"text": chunk.Delta})
			}
		}
		writeEvent("complete", map[string]string{"fullResponse": fullResponse})

		// 7. Persist the assistant response.
		if conversationID != "" && fullResponse != "" {
			if _, err := svcCtx.AICoachRpc.ConversationService.AppendMessage(ctx, &conversationservice.AppendMessageRequest{
				ConversationId: conversationID,
				UserId:         p.UserID,
				Role:           "assistant",
				Content:        fullResponse,
			}); err != nil {
				logx.WithContext(ctx).Errorf("voice turn: persist assistant message: %v", err)
			}
		}

		// 8. Synthesize and send the spoken audio (base64-encoded in the SSE
		// event so it flows through the same stream / BFF proxy cleanly).
		synthCtx, cancel2 := context.WithTimeout(ctx, 30*time.Second)
		synthResp, err := svcCtx.AICoachRpc.AICoachService.Synthesize(synthCtx, &aicoachservice.SynthesizeRequest{
			UserId: p.UserID,
			Text:   fullResponse,
		})
		cancel2()
		if err != nil {
			logx.WithContext(ctx).Errorf("voice turn: synthesize: %v", err)
			// Non-fatal — text already delivered; audio is a nice-to-have.
			writeError("voice synthesis unavailable")
		} else if len(synthResp.Audio) > 0 {
			writeEvent("audio", map[string]string{
				"format": synthResp.Format,
				"data":   base64.StdEncoding.EncodeToString(synthResp.Audio),
			})
		}

		writeEvent("ready", map[string]any{})
		logx.WithContext(ctx).Infof("voice turn complete: user=%s text_len=%d elapsed=%v", p.UserID, len(fullResponse), time.Since(start))
	}
}

// fetchVoiceHistory loads prior conversation messages (excluding the just-
// appended user message) so the LLM has turn context.
func fetchVoiceHistory(ctx context.Context, svcCtx *svc.ServiceContext, conversationID, userID string) []*clientpb.HistoryMessage {
	if conversationID == "" {
		return nil
	}
	resp, err := svcCtx.AICoachRpc.ConversationService.GetMessages(ctx, &conversationservice.GetMessagesRequest{
		ConversationId: conversationID,
		UserId:         userID,
		Page:           1,
		Limit:          50,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("voice turn: fetch history: %v", err)
		return nil
	}
	msgs := resp.Messages
	// Drop the last message if it's the user message we just appended.
	if n := len(msgs); n > 0 && msgs[n-1].Role == "user" {
		msgs = msgs[:n-1]
	}
	history := make([]*clientpb.HistoryMessage, 0, len(msgs))
	for _, m := range msgs {
		history = append(history, &clientpb.HistoryMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return history
}
