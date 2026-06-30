//go:build integration

// Integration test for the weekly review streaming pipeline.
//
// Chain under test:
//
//	Mock AI client (simulates OpenRouter SSE)
//	  → ai-coach RPC (StreamWeeklyReview: real delimiter parsing, delta forwarding)
//	    → mock client RPC (forwards to ai-coach, bypasses Postgres)
//	      → inline SSE HTTP handler (mirrors the real gateway handler)
//	        → Go HTTP client (simulates the frontend EventSource consumer)
//
// Run with:
//
//	go test -tags=integration -v -timeout 120s \
//	  ./services/microservices/ai-coach/rpc/internal/logic/...
//
// No external dependencies (no Postgres, Redis, Kafka, or OpenRouter) required.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/aitest"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	aicoachserver "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/server"
	aicoachsvc "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	aicoachpb "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
	clientpb "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ---------------------------------------------------------------------------
// Local types — mirror the gateway's response types without importing internal.
// ---------------------------------------------------------------------------

type generateWeeklyReviewRequest struct {
	WeekStart       string `json:"weekStart,optional"`
	ForceRegenerate bool   `json:"forceRegenerate,optional"`
}

type weeklyReviewAdjustment struct {
	HabitId        string `json:"habitId"`
	HabitName      string `json:"habitName"`
	AdjustmentType string `json:"adjustmentType"`
	Reason         string `json:"reason"`
	Suggestion     string `json:"suggestion"`
}

type weeklyReviewNextWeekPlan struct {
	Focus           string   `json:"focus"`
	Commitments     []string `json:"commitments"`
	Risks           []string `json:"risks"`
	RecoveryActions []string `json:"recoveryActions"`
}

type weeklyReview struct {
	Id                   string                   `json:"id"`
	UserId               string                   `json:"userId"`
	WeekStart            string                   `json:"weekStart"`
	TotalHabits          int32                    `json:"totalHabits"`
	CompletionRate       float64                  `json:"completionRate"`
	AiSummary            string                   `json:"aiSummary"`
	SuggestedAdjustments []weeklyReviewAdjustment `json:"suggestedAdjustments"`
	NextWeekPlan         weeklyReviewNextWeekPlan `json:"nextWeekPlan"`
	GeneratedAt          string                   `json:"generatedAt"`
}

type weeklyReviewResponse struct {
	Data weeklyReview `json:"data"`
}

// protoToWeeklyReview converts the client RPC proto to our local weeklyReview
// type, mirroring the gateway's ProtoToWeeklyReview conversion.
func protoToWeeklyReview(r *clientpb.WeeklyReview) weeklyReview {
	adjs := make([]weeklyReviewAdjustment, 0, len(r.SuggestedAdjustments))
	for _, a := range r.SuggestedAdjustments {
		adjs = append(adjs, weeklyReviewAdjustment{
			HabitId:        a.HabitId,
			HabitName:      a.HabitName,
			AdjustmentType: a.AdjustmentType,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
		})
	}

	plan := weeklyReviewNextWeekPlan{
		Commitments:     []string{},
		Risks:           []string{},
		RecoveryActions: []string{},
	}
	if r.NextWeekPlan != nil {
		plan.Focus = r.NextWeekPlan.Focus
		if r.NextWeekPlan.Commitments != nil {
			plan.Commitments = r.NextWeekPlan.Commitments
		}
		if r.NextWeekPlan.Risks != nil {
			plan.Risks = r.NextWeekPlan.Risks
		}
		if r.NextWeekPlan.RecoveryActions != nil {
			plan.RecoveryActions = r.NextWeekPlan.RecoveryActions
		}
	}

	return weeklyReview{
		Id:                   r.Id,
		UserId:               r.UserId,
		WeekStart:            r.WeekStart,
		TotalHabits:          r.TotalHabits,
		CompletionRate:       r.CompletionRate,
		AiSummary:            r.AiSummary,
		SuggestedAdjustments: adjs,
		NextWeekPlan:         plan,
		GeneratedAt:          formatUnixTime(r.GeneratedAt),
	}
}

func formatUnixTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// Mock AI client — simulates OpenRouter streaming with the delimiter format.
// ---------------------------------------------------------------------------

// setupMockAIClient creates a MockClient that streams a canned weekly review
// in the delimiter format: summary text, then |||JSON|||, then a JSON object
// with suggestedAdjustments and nextWeekPlan.
func setupMockAIClient() *aitest.MockClient {
	mock := aitest.NewMockClient()

	summary := "Great work this week! You hit a 75% completion rate across your habits. " +
		"Your best day was Wednesday, and the main friction was on Monday mornings. " +
		"Let's build on this momentum next week."

	jsonTail := `{"suggestedAdjustments":[{"habitId":"0194e000-0000-7000-8000-000000000001","habitName":"Morning meditation","adjustmentType":"keep_same","reason":"Strong consistency all week.","suggestion":"Continue your 10-minute morning practice."}],"nextWeekPlan":{"focus":"Protect the streak","commitments":["Meditate 10 minutes each morning","Log before breakfast"],"risks":["Busy Monday could break the streak"],"recoveryActions":["If you miss the morning, do 2 minutes before bed"]}}`

	fullOutput := summary + "\n" + "|||JSON|||" + "\n" + jsonTail

	// Split into small chunks to simulate real token-by-token streaming.
	chunkSize := 8
	runes := []rune(fullOutput)
	var chunks []ai.Chunk
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, ai.Chunk{Delta: string(runes[i:end])})
	}
	// Final chunk with finish reason.
	chunks = append(chunks, ai.Chunk{FinishReason: "stop"})

	mock.RecordStream(ai.ModelCheapLong, chunks...)
	return mock
}

// ---------------------------------------------------------------------------
// Mock client RPC server — implements WeeklyReviewServiceServer by
// forwarding StreamWeeklyReview to the ai-coach RPC, bypassing Postgres.
// ---------------------------------------------------------------------------

type mockClientRPCServer struct {
	clientpb.UnimplementedWeeklyReviewServiceServer
	aiCoachClient aicoachpb.AICoachServiceClient
}

func (s *mockClientRPCServer) StreamWeeklyReview(in *clientpb.GenerateWeeklyReviewRequest, stream clientpb.WeeklyReviewService_StreamWeeklyReviewServer) error {
	ctx := stream.Context()

	// Build a canned WeeklyReviewRequest with hardcoded stats (no DB needed).
	aiReq := &aicoachpb.WeeklyReviewRequest{
		UserId:               in.UserId,
		AccountabilityStyle:  "balanced",
		PreferredTone:        "supportive",
		DifficultyPreference: "adaptive",
		TotalHabits:          3,
		CompletionRate:       75.0,
		CompletedCheckIns:    15,
		MissedCheckIns:       5,
		BestDay:              "Wednesday",
		HardestDay:           "Monday",
		TopBlocker:           "Busy mornings",
		HabitBreakdowns: []*aicoachpb.HabitBreakdown{
			{
				HabitId:        "0194e000-0000-7000-8000-000000000001",
				HabitName:      "Morning meditation",
				Category:       "mindfulness",
				CompletedCount: 6,
				MissedCount:    1,
				CompletionRate: 85.7,
			},
		},
	}

	aiStream, err := s.aiCoachClient.StreamWeeklyReview(ctx, aiReq)
	if err != nil {
		return fmt.Errorf("ai-coach stream open: %w", err)
	}

	for {
		chunk, recvErr := aiStream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				return nil
			}
			return fmt.Errorf("ai-coach stream recv: %w", recvErr)
		}

		if chunk.Complete && chunk.Review != nil {
			// Convert ai-coach review to client review and send as complete.
			return stream.Send(&clientpb.WeeklyReviewStreamChunk{
				Complete: true,
				Review: &clientpb.WeeklyReview{
					Id:                  "test-review-id",
					UserId:              in.UserId,
					WeekStart:           "2026-06-22",
					TotalHabits:         3,
					CompletionRate:      75.0,
					AiSummary:           chunk.Review.AiSummary,
					GeneratedAt:         time.Now().Unix(),
					SuggestedAdjustments: toClientAdjustments(chunk.Review.SuggestedAdjustments),
					NextWeekPlan:        toClientPlan(chunk.Review.NextWeekPlan),
				},
			})
		}

		if chunk.Finalizing {
			if err := stream.Send(&clientpb.WeeklyReviewStreamChunk{Finalizing: true}); err != nil {
				return err
			}
			continue
		}

		if chunk.Delta != "" {
			if err := stream.Send(&clientpb.WeeklyReviewStreamChunk{Delta: chunk.Delta}); err != nil {
				return err
			}
		}
	}
}

func toClientAdjustments(adjs []*aicoachpb.WeeklyReviewAdjustment) []*clientpb.WeeklyReviewAdjustment {
	out := make([]*clientpb.WeeklyReviewAdjustment, len(adjs))
	for i, a := range adjs {
		out[i] = &clientpb.WeeklyReviewAdjustment{
			HabitId:        a.HabitId,
			HabitName:      a.HabitName,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
			AdjustmentType: a.AdjustmentType,
		}
	}
	return out
}

func toClientPlan(p *aicoachpb.NextWeekPlan) *clientpb.WeeklyReviewNextWeekPlan {
	if p == nil {
		return &clientpb.WeeklyReviewNextWeekPlan{}
	}
	return &clientpb.WeeklyReviewNextWeekPlan{
		Focus:           p.Focus,
		Commitments:     p.Commitments,
		Risks:           p.Risks,
		RecoveryActions: p.RecoveryActions,
	}
}

// ---------------------------------------------------------------------------
// Server startup helpers.
// ---------------------------------------------------------------------------

// freePort returns a random available TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// startAICoachServer starts the ai-coach gRPC server with a mock AI client.
// Returns the listen address.
func startAICoachServer(t *testing.T, mockAI ai.Client) string {
	t.Helper()
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	svcCtx := &aicoachsvc.ServiceContext{
		AIClient: mockAI,
	}

	s := zrpc.MustNewServer(zrpc.RpcServerConf{
		ServiceConf: service.ServiceConf{
			Mode: service.TestMode,
		},
		ListenOn: addr,
	}, func(grpcServer *grpc.Server) {
		aicoachpb.RegisterAICoachServiceServer(grpcServer, aicoachserver.NewAICoachServiceServer(svcCtx))
	})

	t.Cleanup(func() { s.Stop() })

	go s.Start()
	// Give the server a moment to bind.
	time.Sleep(100 * time.Millisecond)
	return addr
}

// startMockClientRPC starts a mock client RPC server that forwards
// StreamWeeklyReview to the ai-coach RPC. Returns the listen address.
func startMockClientRPC(t *testing.T, aiCoachAddr string) string {
	t.Helper()
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Connect to ai-coach RPC.
	conn, err := grpc.NewClient(aiCoachAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial ai-coach: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	aiCoachClient := aicoachpb.NewAICoachServiceClient(conn)
	server := &mockClientRPCServer{aiCoachClient: aiCoachClient}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen client rpc: %v", err)
	}

	grpcServer := grpc.NewServer()
	clientpb.RegisterWeeklyReviewServiceServer(grpcServer, server)

	t.Cleanup(func() { grpcServer.GracefulStop() })

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("client rpc server stopped: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	return addr
}

// startGatewayHTTP starts an HTTP server with an inline SSE handler that
// mirrors the real gateway's streamWeeklyReviewHandler. Returns the base URL.
func startGatewayHTTP(t *testing.T, clientRPCAddr string) string {
	t.Helper()

	// Connect to the mock client RPC.
	conn, err := grpc.NewClient(clientRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial client rpc: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	wrClient := clientpb.NewWeeklyReviewServiceClient(conn)

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	baseURL := fmt.Sprintf("http://%s", addr)

	handler := buildSSEHandler(wrClient)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/weekly-reviews/generate-stream", handler)

	httpServer := &http.Server{Addr: addr, Handler: mux}
	t.Cleanup(func() { httpServer.Close() })

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("gateway http server stopped: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	return baseURL
}

// buildSSEHandler creates an http.HandlerFunc that mirrors the real gateway
// SSE handler (streamWeeklyReviewHandler.go) but uses a raw gRPC client
// instead of the go-zero ServiceContext. Auth is bypassed with a fake
// principal.
func buildSSEHandler(wrClient clientpb.WeeklyReviewServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Inject a fake principal (bypass JWT auth).
		ctx := principal.WithPrincipal(r.Context(), principal.Principal{
			UserID:   "019eca3a-a221-74d5-8e8e-40723e5d2d80",
			Username: "test-user",
		})

		var req generateWeeklyReviewRequest
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			if len(body) > 0 {
				_ = json.Unmarshal(body, &req)
			}
		}

		p, _ := principal.PrincipalFrom(ctx)

		stream, err := wrClient.StreamWeeklyReview(ctx, &clientpb.GenerateWeeklyReviewRequest{
			UserId:          p.UserID,
			WeekStart:       req.WeekStart,
			ForceRegenerate: req.ForceRegenerate,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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

		for {
			chunk, recvErr := stream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					fmt.Fprintf(w, "event: error\ndata: {\"message\":\"stream ended before completion\"}\n\n")
					flush()
					return
				}
				errMsg, _ := json.Marshal(map[string]string{"message": recvErr.Error()})
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
				flush()
				return
			}

			if chunk.Complete {
				if chunk.Review == nil {
					fmt.Fprintf(w, "event: error\ndata: {\"message\":\"stream completed without a review\"}\n\n")
					flush()
					return
				}
				resp := &weeklyReviewResponse{
					Data: protoToWeeklyReview(chunk.Review),
				}
				data, _ := json.Marshal(resp)
				fmt.Fprintf(w, "event: complete\ndata: %s\n\n", data)
				flush()
				return
			}

			if chunk.Finalizing {
				fmt.Fprintf(w, "event: finalizing\ndata: {}\n\n")
				flush()
				continue
			}

			if chunk.Delta == "" {
				fmt.Fprintf(w, ": keepalive\n\n")
				flush()
				continue
			}

			deltaData, _ := json.Marshal(map[string]string{"text": chunk.Delta})
			fmt.Fprintf(w, "event: delta\ndata: %s\n\n", deltaData)
			flush()
		}
	}
}

// ---------------------------------------------------------------------------
// SSE client — simulates the frontend EventSource consumer.
// ---------------------------------------------------------------------------

type sseEvent struct {
	Type string
	Data string
}

// readSSEStream reads SSE events from an HTTP response body, calling onEvent
// for each parsed event. Returns when the stream ends.
func readSSEStream(t *testing.T, body io.ReadCloser, onEvent func(sseEvent)) {
	t.Helper()
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventType string
	var dataLines []string

	flushEvent := func() {
		if eventType == "" && len(dataLines) == 0 {
			return
		}
		ev := sseEvent{
			Type: eventType,
			Data: strings.Join(dataLines, "\n"),
		}
		if ev.Type == "" {
			ev.Type = "message"
		}
		onEvent(ev)
		eventType = ""
		dataLines = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flushEvent()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // comment/keepalive
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(line[5:]))
		}
	}
	flushEvent()

	if err := scanner.Err(); err != nil {
		t.Logf("scanner error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test.
// ---------------------------------------------------------------------------

// TestStreamWeeklyReviewIntegration tests the full streaming pipeline:
// mock OpenRouter → ai-coach RPC → mock client RPC → HTTP/SSE → Go client.
//
// It verifies:
//   - Summary text is streamed as delta events in real-time (not buffered)
//   - The |||JSON||| delimiter does NOT leak into the delta stream
//   - The JSON tail (structured fields) does NOT leak into deltas
//   - A finalizing event is sent after the summary text
//   - A complete event contains the full review with structured fields
//   - The complete review's AiSummary matches the streamed text
func TestStreamWeeklyReviewIntegration(t *testing.T) {
	// 1. Setup mock AI client that simulates OpenRouter streaming.
	mockAI := setupMockAIClient()

	// 2. Start ai-coach RPC server with the mock AI client.
	aiCoachAddr := startAICoachServer(t, mockAI)
	t.Logf("ai-coach RPC listening on %s", aiCoachAddr)

	// 3. Start mock client RPC that forwards to ai-coach.
	clientRPCAddr := startMockClientRPC(t, aiCoachAddr)
	t.Logf("client RPC listening on %s", clientRPCAddr)

	// 4. Start gateway HTTP server with SSE handler.
	baseURL := startGatewayHTTP(t, clientRPCAddr)
	t.Logf("gateway HTTP listening on %s", baseURL)

	// 5. Make the SSE request (simulating the frontend fetch).
	reqBody := `{"weekStart":"2026-06-22"}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		baseURL+"/api/v1/weekly-reviews/generate-stream",
		strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream Content-Type, got %q", ct)
	}

	// 6. Consume the SSE stream and collect events.
	var deltas []string
	var gotFinalizing bool
	var completeReview *weeklyReviewResponse
	var errorMsg string

	readSSEStream(t, resp.Body, func(ev sseEvent) {
		t.Logf("[SSE] event=%s data=%s", ev.Type, truncateStr(ev.Data, 120))

		switch ev.Type {
		case "delta":
			var payload struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(ev.Data), &payload); err != nil {
				t.Errorf("parse delta: %v", err)
				return
			}
			deltas = append(deltas, payload.Text)

		case "finalizing":
			gotFinalizing = true

		case "complete":
			var payload weeklyReviewResponse
			if err := json.Unmarshal([]byte(ev.Data), &payload); err != nil {
				t.Errorf("parse complete: %v", err)
				return
			}
			completeReview = &payload

		case "error":
			var payload struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal([]byte(ev.Data), &payload)
			errorMsg = payload.Message
		}
	})

	// 7. Assertions.

	if errorMsg != "" {
		t.Fatalf("received SSE error event: %s", errorMsg)
	}

	if len(deltas) == 0 {
		t.Fatal("expected at least one delta event, got none")
	}

	// The streamed deltas should reconstruct the summary text (before the
	// |||JSON||| delimiter). The delimiter and JSON tail should NOT appear
	// in the deltas — they are consumed by the ai-coach logic.
	streamedText := strings.Join(deltas, "")
	t.Logf("streamed summary text (%d chars): %s", len(streamedText), truncateStr(streamedText, 200))

	if strings.Contains(streamedText, "|||JSON|||") {
		t.Errorf("delimiter leaked into delta stream: %q", streamedText)
	}
	if strings.Contains(streamedText, "suggestedAdjustments") {
		t.Errorf("JSON tail leaked into delta stream: %q", streamedText)
	}

	// The streamed text should contain the expected summary content.
	expectedSummary := "Great work this week!"
	if !strings.Contains(streamedText, expectedSummary) {
		t.Errorf("streamed text %q does not contain expected summary %q",
			truncateStr(streamedText, 100), expectedSummary)
	}

	if !gotFinalizing {
		t.Error("expected a finalizing event before complete, got none")
	}

	if completeReview == nil {
		t.Fatal("expected a complete event with the review, got none")
	}

	t.Logf("complete review: aiSummary=%q adjustments=%d focus=%q",
		truncateStr(completeReview.Data.AiSummary, 100),
		len(completeReview.Data.SuggestedAdjustments),
		completeReview.Data.NextWeekPlan.Focus)

	// The complete review's AiSummary should match the streamed text.
	// The ai-coach logic trims trailing whitespace from the summary, so we
	// compare trimmed versions (the streamed deltas may include the \n that
	// preceded the |||JSON||| delimiter).
	if strings.TrimSpace(completeReview.Data.AiSummary) != strings.TrimSpace(streamedText) {
		t.Errorf("complete review AiSummary %q does not match streamed text %q",
			truncateStr(completeReview.Data.AiSummary, 100),
			truncateStr(streamedText, 100))
	}

	// The complete review should have structured fields from the JSON tail.
	if len(completeReview.Data.SuggestedAdjustments) == 0 {
		t.Error("expected at least one suggested adjustment in the complete review")
	} else {
		adj := completeReview.Data.SuggestedAdjustments[0]
		if adj.HabitName != "Morning meditation" {
			t.Errorf("expected adjustment habit name 'Morning meditation', got %q", adj.HabitName)
		}
		if adj.AdjustmentType != "keep_same" {
			t.Errorf("expected adjustment type 'keep_same', got %q", adj.AdjustmentType)
		}
	}

	if completeReview.Data.NextWeekPlan.Focus != "Protect the streak" {
		t.Errorf("expected next week plan focus 'Protect the streak', got %q",
			completeReview.Data.NextWeekPlan.Focus)
	}

	if len(completeReview.Data.NextWeekPlan.Commitments) != 2 {
		t.Errorf("expected 2 commitments, got %d", len(completeReview.Data.NextWeekPlan.Commitments))
	}

	t.Logf("✓ Integration test passed: %d deltas, finalizing=%v, complete review with %d adjustments",
		len(deltas), gotFinalizing, len(completeReview.Data.SuggestedAdjustments))
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
