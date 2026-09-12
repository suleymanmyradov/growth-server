package weeklyreview

import (
	"io"
	"net/http"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/logic/weeklyreview"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/sse"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	clientweeklyreview "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// thinkingMessages are sent to the client as SSE "thinking" events while the
// model is processing before the first token arrives. They cycle through
// contextually relevant messages so the user sees activity during the
// (potentially long) time-to-first-token period.
var thinkingMessages = []string{
	"Analyzing your week...",
	"Reviewing your habits and check-ins...",
	"Looking at your completion patterns...",
	"Identifying what worked and what didn't...",
	"Considering your blockers and challenges...",
	"Reflecting on your mood and energy trends...",
	"Crafting your personalized review...",
}

// StreamWeeklyReviewHandler is an SSE endpoint that streams the AI-generated
// weekly review summary text as it is produced, then sends a final "complete"
// event with the full persisted review object.
//
// SSE event types:
//   - thinking:  {"message": "..."} — status updates while the model processes
//   - delta:     {"text": "..."} — incremental summary text
//   - finalizing: {} — summary text done, parsing structured JSON
//   - complete:  {full WeeklyReviewResponse} — final review after persistence
//   - error:     {"message": "..."} — error before stream end
func StreamWeeklyReviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GenerateWeeklyReviewRequest
		if err := httpx.Parse(r, &req); err != nil {
			errors.WriteParseError(w, err)
			return
		}

		p, ok := principal.PrincipalFrom(r.Context())
		if !ok {
			errors.HandleGrpcError(w, status.Error(codes.Unauthenticated, "not authenticated"))
			return
		}

		streamStart := time.Now()

		// Step 1: Prepare — compute stats, check cache/cooldown.
		logx.WithContext(r.Context()).Infof("SSE stream: preparing weekly review for user=%s weekStart=%s", p.UserID, req.WeekStart)
		prepResp, err := svcCtx.ClientRpc.WeeklyReviewService.PrepareWeeklyReview(r.Context(), &clientweeklyreview.PrepareWeeklyReviewRequest{
			UserId:          p.UserID,
			WeekStart:       req.WeekStart,
			ForceRegenerate: req.ForceRegenerate,
		})
		if err != nil {
			logx.WithContext(r.Context()).Errorf("SSE stream: prepare failed after %v: %v", time.Since(streamStart), err)
			errors.HandleGrpcError(w, err)
			return
		}

		// If a cached review exists, return it directly (no AI call needed).
		if prepResp.ExistingReview != nil {
			logx.WithContext(r.Context()).Infof("SSE stream: returning cached review")
			sseWriter := sse.NewWriter(w)
			resp := &types.WeeklyReviewResponse{Data: weeklyreview.ProtoToWeeklyReview(prepResp.ExistingReview)}
			sseWriter.WriteEvent("complete", resp)
			return
		}

		if prepResp.Data == nil {
			errors.HandleGrpcError(w, status.Error(codes.Internal, "prepare returned no data and no existing review"))
			return
		}

		// Plan-aware daily token cap, enforced at the edge before any AI work.
		// Placed after the cached-review return so existing reviews stay
		// readable even when the user is capped out.
		if err := svcCtx.CheckDailyTokenQuota(r.Context(), p.UserID); err != nil {
			sse.NewWriter(w).WriteError(ai.UserFacingMessage(err))
			return
		}

		// Step 2: Open the ai-coach streaming RPC directly.
		aiReq := weeklyreview.BuildWeeklyReviewAIRequest(prepResp.Data)
		logx.WithContext(r.Context()).Infof("SSE stream: opening ai-coach stream for user=%s", p.UserID)
		aiStream, err := svcCtx.AICoachRpc.AICoachService.StreamWeeklyReview(r.Context(), aiReq)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("SSE stream: ai-coach stream open failed after %v: %v", time.Since(streamStart), err)
			errors.HandleGrpcError(w, err)
			return
		}
		logx.WithContext(r.Context()).Infof("SSE stream: ai-coach stream opened after %v", time.Since(streamStart))

		// Commit SSE headers and create the thread-safe SSE writer.
		sseWriter := sse.NewWriter(w)

		// Thinking goroutine: sends periodic "thinking" SSE events while the
		// model processes the prompt. Stops on first delta or stream end.
		thinking := sse.NewThinkingGuard(sseWriter, thinkingMessages, 2*time.Second)
		thinking.Start()
		defer thinking.Stop()

		// Heartbeat goroutine: sends SSE comments every 15s to keep the
		// connection alive while the AI generates structured JSON (can take
		// 60+ seconds with no output). This was previously in the client RPC.
		heartbeat := sse.NewHeartbeatGuard(sseWriter, 15*time.Second)
		heartbeat.Start()
		defer heartbeat.Stop()

		deltaCount := 0
		var totalDeltaChars int

		// Step 3: Relay chunks from the ai-coach stream to the SSE client.
		var aiSummary string
		var suggestedAdjustments []*aicoachservice.WeeklyReviewAdjustment
		var nextWeekPlan *aicoachservice.NextWeekPlan

		for {
			chunk, recvErr := aiStream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					logx.WithContext(r.Context()).Errorf("SSE stream: upstream EOF after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
					sseWriter.WriteError("stream ended before completion")
					return
				}
				logx.WithContext(r.Context()).Errorf("SSE stream: upstream recv error after %d deltas, %d chars, %v elapsed: %v", deltaCount, totalDeltaChars, time.Since(streamStart), recvErr)
				sseWriter.WriteError(grpcErrMsg(recvErr))
				return
			}

			if chunk.Complete {
				if chunk.Review != nil {
					aiSummary = chunk.Review.AiSummary
					suggestedAdjustments = chunk.Review.SuggestedAdjustments
					nextWeekPlan = chunk.Review.NextWeekPlan
				}
				break
			}

			if chunk.Finalizing {
				logx.WithContext(r.Context()).Infof("SSE stream: finalizing event after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
				sseWriter.WriteRaw("finalizing", "{}")
				continue
			}

			if chunk.Delta != "" {
				thinking.SignalFirstEvent()
				deltaCount++
				totalDeltaChars += len(chunk.Delta)
				if !sseWriter.WriteEvent("delta", map[string]string{"text": chunk.Delta}) {
					logx.WithContext(r.Context()).Errorf("SSE stream: write delta error after %d deltas", deltaCount)
					return
				}
			}
		}

		// Stop the heartbeat before persisting (thinking already stopped
		// via SignalFirstEvent on the first delta, or is a no-op if no
		// deltas arrived).
		heartbeat.Stop()

		// Step 4: Save — persist to DB via the client RPC.
		saveResp, err := svcCtx.ClientRpc.WeeklyReviewService.SaveWeeklyReview(r.Context(), &clientweeklyreview.SaveWeeklyReviewRequest{
			Data:                 prepResp.Data,
			AiSummary:            aiSummary,
			SuggestedAdjustments: weeklyreview.ConvertAdjustmentsToClient(suggestedAdjustments),
			NextWeekPlan:         weeklyreview.ConvertNextWeekPlanToClient(nextWeekPlan),
		})
		if err != nil {
			logx.WithContext(r.Context()).Errorf("SSE stream: save failed: %v", err)
			sseWriter.WriteError("failed to save weekly review")
			return
		}

		// Send the final complete event with the persisted review.
		resp := &types.WeeklyReviewResponse{Data: weeklyreview.ProtoToWeeklyReview(saveResp.Review)}
		sseWriter.WriteEvent("complete", resp)
		logx.WithContext(r.Context()).Infof("SSE stream: complete event sent after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
	}
}

func grpcErrMsg(err error) string {
	// Map known ai package errors to user-friendly messages; for everything
	// else (gRPC transport errors, etc.) return a generic fallback so raw
	// internal error text never reaches the UI.
	return ai.UserFacingMessage(err)
}
