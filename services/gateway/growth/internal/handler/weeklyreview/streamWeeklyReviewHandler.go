package weeklyreview

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/weeklyreview"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
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

		// Open the client-rpc streaming RPC before writing any response, so a
		// dial-time error can be surfaced as a proper HTTP status code rather
		// than an in-band SSE error event after a committed 200.
		streamStart := time.Now()
		logx.WithContext(r.Context()).Infof("SSE stream: opening client-rpc stream for user=%s weekStart=%s", p.UserID, req.WeekStart)
		stream, err := svcCtx.WeeklyReviewRpc.StreamWeeklyReview(r.Context(), &clientweeklyreview.GenerateWeeklyReviewRequest{
			UserId:          p.UserID,
			WeekStart:       req.WeekStart,
			ForceRegenerate: req.ForceRegenerate,
		})
		if err != nil {
			logx.WithContext(r.Context()).Errorf("SSE stream: client-rpc stream open failed after %v: %v", time.Since(streamStart), err)
			errors.HandleGrpcError(w, err)
			return
		}
		logx.WithContext(r.Context()).Infof("SSE stream: client-rpc stream opened after %v", time.Since(streamStart))

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
		// while the model processes the prompt. The goroutine stops once the
		// first delta arrives (firstDelta signal) or the stream ends (done
		// channel). This gives the user visual feedback during the
		// potentially long time-to-first-token period.
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
				if chunk.Review == nil {
					logx.WithContext(r.Context()).Errorf("SSE stream: complete chunk has nil review after %d deltas, %v elapsed", deltaCount, time.Since(streamStart))
					writeSSEError(w, flush, "stream completed without a review")
					return
				}
				resp := &types.WeeklyReviewResponse{Data: weeklyreview.ProtoToWeeklyReview(chunk.Review)}
				data, _ := json.Marshal(resp)
				if _, err := fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data); err != nil {
					return
				}
				flush()
				logx.WithContext(r.Context()).Infof("SSE stream: complete event sent after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
				return
			}

			if chunk.Finalizing {
				logx.WithContext(r.Context()).Infof("SSE stream: finalizing event after %d deltas, %d chars, %v elapsed", deltaCount, totalDeltaChars, time.Since(streamStart))
				if _, err := fmt.Fprintf(w, "event: finalizing\ndata: {}\n\n"); err != nil {
					return
				}
				flush()
				continue
			}

			// Empty delta is a heartbeat keepalive — send an SSE comment
			// and flush to keep the connection alive without producing a
			// client-visible event.
			if chunk.Delta == "" {
				if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
					return
				}
				flush()
				continue
			}

			if chunk.Delta != "" {
				// Signal the thinking goroutine to stop on the first real delta.
				if !firstDeltaSent {
					firstDeltaSent = true
					close(firstDelta)
				}
				deltaCount++
				totalDeltaChars += len(chunk.Delta)
				deltaData, _ := json.Marshal(map[string]string{"text": chunk.Delta})
				if _, err := fmt.Fprintf(w, "event: delta\ndata: %s\n\n", deltaData); err != nil {
					logx.WithContext(r.Context()).Errorf("SSE stream: write delta error after %d deltas: %v", deltaCount, err)
					return
				}
				flush()
			}
		}
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
