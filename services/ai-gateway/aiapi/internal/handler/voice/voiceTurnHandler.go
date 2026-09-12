package voice

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/logic/personalization"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/sse"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SSE event types for POST /personalization/voice-turn:
//   transcript  {"text":"...","language":"...","duration":...}  — STT result
//   reasoning   {"text":"..."}                                  — model reasoning delta (reasoning models only)
//   thinking    {"message":"..."}                               — status update while tools execute
//   delta       {"text":"..."}                                  — coaching text delta
//   proposal    {id, action, payload}                           — proposed CRUD action (confirm/cancel card)
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
// SSE stream that carries the transcription, the streamed agentic coaching
// response, and the synthesized spoken audio. This is the live voice chat
// transport — turn-based: the client records one utterance, uploads it, and
// plays back the spoken response. It reuses the existing BFF proxy (SSE
// streams through).
//
// Coaching uses the same agentic flow as the text coaching-stream endpoint
// (StreamAgent with on-demand tool calls). The voice handler wraps it with
// STT transcription before and TTS synthesis after.
func VoiceTurnHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := principal.PrincipalFrom(r.Context())
		if !ok {
			errors.HandleGrpcError(w, status.Error(codes.Unauthenticated, "not authenticated"))
			return
		}

		// Plan-aware daily token cap, enforced at the edge before any AI work
		// (including STT, which is not token-metered but always precedes the
		// coaching turn).
		if err := svcCtx.CheckDailyTokenQuota(r.Context(), p.UserID); err != nil {
			sse.NewWriter(w).WriteError(ai.UserFacingMessage(err))
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
		defer func() { _ = file.Close() }()

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
		sseWriter := sse.NewWriter(w)

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
			sseWriter.WriteError("I couldn't understand your audio. Please try speaking again.")
			return
		}
		userText := transResp.Text
		if userText == "" {
			sseWriter.WriteError("transcription was empty")
			return
		}
		sseWriter.WriteEvent("transcript", map[string]any{
			"text":     userText,
			"language": transResp.Language,
			"duration": transResp.Duration,
		})

		// 2. Ensure a conversation exists. For a new conversation, create
		// it WITHOUT an initial message — StreamCoaching's
		// fetchAndPersistHistory will append the user message and fetch
		// prior history. For an existing conversation, StreamCoaching
		// handles history fetch + user message append.
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
			}
		}

		// 3. Inform the client which conversation the turn is persisted to
		// so the text view can be refreshed with the full turn. Sent before
		// coaching so the URL updates immediately.
		if conversationID != "" {
			sseWriter.WriteEvent("conversation", map[string]string{"id": conversationID})
		}

		// 4. Run the agentic coaching flow (safety classification, history,
		// tools, StreamAgent, SSE forwarding, assistant message persist).
		// StreamCoaching writes delta/complete/error events directly to the
		// shared SSE writer and returns the full response text.
		fullResponse := personalization.StreamCoaching(ctx, sseWriter, &types.GeneratePersonalizedCoachingRequest{
			UserMessage:    userText,
			ConversationId: conversationID,
		}, p, personalization.StreamCoachingDeps{
			AIClient:       svcCtx.AIClient,
			Classifier:     svcCtx.Classifier,
			Conversations:  svcCtx.AICoachRpc.ConversationService,
			ProfileFetcher: svcCtx.AuthRpc,
			ToolDeps: personalization.CoachingToolDeps{
				Goals:           svcCtx.ClientRpc.Goals,
				Habits:          svcCtx.ClientRpc.Habits,
				CheckIns:        svcCtx.ClientRpc.CheckInService,
				WeeklyReviews:   svcCtx.ClientRpc.WeeklyReviewService,
				Personalization: svcCtx.ClientRpc.PersonalizationService,
				Search:          svcCtx.SearchRpc,
				Articles:        svcCtx.ClientRpc.Articles,
				Memory:          svcCtx.AICoachRpc.AICoachService,
			},
			Config: svcCtx.Config.Coaching,
		})

		// 5. Synthesize and send the spoken audio (base64-encoded in the SSE
		// event so it flows through the same stream / BFF proxy cleanly).
		// Skipped when coaching produced no response (error or empty).
		if fullResponse != "" {
			synthCtx, cancel2 := context.WithTimeout(ctx, 30*time.Second)
			synthResp, err := svcCtx.AICoachRpc.AICoachService.Synthesize(synthCtx, &aicoachservice.SynthesizeRequest{
				UserId: p.UserID,
				Text:   fullResponse,
			})
			cancel2()
			if err != nil {
				logx.WithContext(ctx).Errorf("voice turn: synthesize: %v", err)
				// Non-fatal — text already delivered; audio is a nice-to-have.
				sseWriter.WriteError("voice synthesis unavailable")
			} else if len(synthResp.Audio) > 0 {
				sseWriter.WriteEvent("audio", map[string]string{
					"format": synthResp.Format,
					"data":   base64.StdEncoding.EncodeToString(synthResp.Audio),
				})
			}
		}

		sseWriter.WriteEvent("ready", map[string]any{})
		logx.WithContext(ctx).Infof("voice turn complete: user=%s text_len=%d elapsed=%v", p.UserID, len(fullResponse), time.Since(start))
	}
}
