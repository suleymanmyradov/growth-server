package weeklyreview

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/weeklyreview"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
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
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			resp := &types.WeeklyReviewResponse{Data: weeklyreview.ProtoToWeeklyReview(prepResp.ExistingReview)}
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
			return
		}

		if prepResp.Data == nil {
			errors.HandleGrpcError(w, status.Error(codes.Internal, "prepare returned no data and no existing review"))
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

		// Serialize all SSE writes (thinking goroutine, heartbeat, and main loop).
		var writeMu sync.Mutex
		writeSSE := func(event, data string) error {
			writeMu.Lock()
			defer writeMu.Unlock()
			_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			if err != nil {
				return err
			}
			flush()
			return nil
		}
		writeKeepalive := func() error {
			writeMu.Lock()
			defer writeMu.Unlock()
			_, err := fmt.Fprintf(w, ": keepalive\n\n")
			if err != nil {
				return err
			}
			flush()
			return nil
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
					msg := thinkingMessages[idx%len(thinkingMessages)]
					idx++
					data, _ := json.Marshal(map[string]string{"message": msg})
					_ = writeSSE("thinking", string(data))
				}
			}
		}()

		// Heartbeat goroutine: sends SSE comments every 15s to keep the
		// connection alive while the AI generates structured JSON (can take
		// 60+ seconds with no output). This was previously in the client RPC.
		heartbeatDone := make(chan struct{})
		var heartbeatWG sync.WaitGroup
		heartbeatWG.Add(1)
		go func() {
			defer heartbeatWG.Done()
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-heartbeatDone:
					return
				case <-ticker.C:
					_ = writeKeepalive()
				}
			}
		}()
		defer func() {
			close(heartbeatDone)
			heartbeatWG.Wait()
		}()

		deltaCount := 0
		var totalDeltaChars int
		firstDeltaSent := false

		// Step 3: Relay chunks from the ai-coach stream to the SSE client.
		var aiSummary string
		var suggestedAdjustments []*aicoachservice.WeeklyReviewAdjustment
		var nextWeekPlan *aicoachservice.NextWeekPlan

		for {
			chunk, recvErr := aiStream.Recv()
			if recvErr != nil {
				close(done)
				if recvErr == io.EOF {
					logx.WithContext(r.Context()).Errorf("SSE stream: upstream EOF after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
					writeSSEError(w, flush, "stream ended before completion")
					return
				}
				logx.WithContext(r.Context()).Errorf("SSE stream: upstream recv error after %d deltas, %d chars, %v elapsed: %v", deltaCount, totalDeltaChars, time.Since(streamStart), recvErr)
				writeSSEError(w, flush, grpcErrMsg(recvErr))
				return
			}

			if chunk.Complete {
				close(done)
				if chunk.Review != nil {
					aiSummary = chunk.Review.AiSummary
					suggestedAdjustments = chunk.Review.SuggestedAdjustments
					nextWeekPlan = chunk.Review.NextWeekPlan
				}
				break
			}

			if chunk.Finalizing {
				logx.WithContext(r.Context()).Infof("SSE stream: finalizing event after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
				writeMu.Lock()
				fmt.Fprintf(w, "event: finalizing\ndata: {}\n\n")
				flush()
				writeMu.Unlock()
				continue
			}

			if chunk.Delta != "" {
				if !firstDeltaSent {
					firstDeltaSent = true
					close(firstDelta)
				}
				deltaCount++
				totalDeltaChars += len(chunk.Delta)
				deltaData, _ := json.Marshal(map[string]string{"text": chunk.Delta})
				writeMu.Lock()
				if _, err := fmt.Fprintf(w, "event: delta\ndata: %s\n\n", deltaData); err != nil {
					writeMu.Unlock()
					logx.WithContext(r.Context()).Errorf("SSE stream: write delta error after %d deltas: %v", deltaCount, err)
					return
				}
				flush()
				writeMu.Unlock()
			}
		}

		// Stop the heartbeat before persisting.
		close(heartbeatDone)
		heartbeatWG.Wait()

		// Step 4: Save — persist to DB via the client RPC.
		saveResp, err := svcCtx.ClientRpc.WeeklyReviewService.SaveWeeklyReview(r.Context(), &clientweeklyreview.SaveWeeklyReviewRequest{
			Data:                 prepResp.Data,
			AiSummary:            aiSummary,
			SuggestedAdjustments: weeklyreview.ConvertAdjustmentsToClient(suggestedAdjustments),
			NextWeekPlan:         weeklyreview.ConvertNextWeekPlanToClient(nextWeekPlan),
		})
		if err != nil {
			logx.WithContext(r.Context()).Errorf("SSE stream: save failed: %v", err)
			writeSSEError(w, flush, "failed to save weekly review")
			return
		}

		// Send the final complete event with the persisted review.
		resp := &types.WeeklyReviewResponse{Data: weeklyreview.ProtoToWeeklyReview(saveResp.Review)}
		data, _ := json.Marshal(resp)
		writeMu.Lock()
		fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
		flush()
		writeMu.Unlock()
		logx.WithContext(r.Context()).Infof("SSE stream: complete event sent after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
	}
}

func writeSSEError(w http.ResponseWriter, flush func(), msg string) {
	data, _ := json.Marshal(map[string]string{"message": msg})
	if _, err := fmt.Fprintf(w, "event: error\ndata: %s\n\n", data); err != nil {
		return
	}
	flush()
}

func grpcErrMsg(err error) string {
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	return err.Error()
}
