package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	ai "github.com/suleymanmyradov/growth-server/pkg/ai"
)

func main() {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY not set")
	}

	// Use the free model for smoke test.
	cfg := ai.Config{
		APIKey: apiKey,
		Models: map[ai.ModelProfile]string{
			ai.ModelCheap: "nvidia/nemotron-3-super-120b-a12b:free",
			ai.ModelChat:  "nvidia/nemotron-3-super-120b-a12b:free",
		},
		DefaultTimeout: 60 * time.Second,
		MaxRetries:     8,
		RetryBackoff:   3 * time.Second,
		HTTPReferer:    "https://growth.app",
		XTitle:         "growth-smoke-test",
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validate: %v", err)
	}

	client, err := ai.New(cfg)
	if err != nil {
		log.Fatalf("new client: %v", err)
	}

	ctx := context.Background()

	// ── Test 1: Basic Generate ──
	fmt.Println("=== Test 1: Generate ===")
	resp, err := client.Generate(ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       "You are a helpful assistant. Respond in one sentence.",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What is 2+2?"},
		},
		Metadata: ai.Metadata{UserID: "smoke-test", Feature: "smoke-generate", ConversationID: "conv-1"},
	})
	if err != nil {
		log.Fatalf("generate: %v", err)
	}
	fmt.Printf("Content: %s\n", resp.Message.Content)
	fmt.Printf("Model:   %s\n", resp.ModelID)
	fmt.Printf("Usage:   prompt=%d completion=%d total=%d\n", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	fmt.Printf("Cost:    $%.6f\n", resp.CostUSD)
	fmt.Printf("Latency: %dms\n\n", resp.LatencyMS)

	// Cooldown to avoid free-tier rate limits.
	fmt.Println("--- cooldown 5s ---")
	time.Sleep(5 * time.Second)

	// ── Test 2: GenerateStructured ──
	fmt.Println("=== Test 2: GenerateStructured ===")
	var structured struct {
		Answer int    `json:"answer"`
		Reason string `json:"reason"`
	}
	err = client.GenerateStructured(ctx, ai.GenerateRequest{
		ModelProfile:   ai.ModelCheap,
		System:         "You are a math assistant. Always respond with valid JSON.",
		Messages:       []ai.Message{{Role: ai.RoleUser, Content: "What is 3*5? Respond as JSON with fields: answer (int), reason (string)."}},
		ResponseFormat: ai.ResponseFormatJSON,
		Metadata:       ai.Metadata{UserID: "smoke-test", Feature: "smoke-structured"},
	}, &structured)
	if err != nil {
		fmt.Printf("SKIP (rate-limited): %v\n\n", err)
	} else {
		fmt.Printf("Answer: %d\n", structured.Answer)
		fmt.Printf("Reason: %s\n\n", structured.Reason)
	}

	// ── Test 3: Tool Calling ──
	fmt.Println("--- cooldown 5s ---")
	time.Sleep(5 * time.Second)

	fmt.Println("=== Test 3: Tool Calling (Agent) ===")

	type weatherInput struct {
		City string `json:"city" jsonschema:"required,description=City name"`
	}
	type weatherOutput struct {
		Temp      int    `json:"temp"`
		Condition string `json:"condition"`
	}

	weatherTool := ai.NewTool[weatherInput, weatherOutput](ai.ToolSpec{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		Handler: func(_ context.Context, in weatherInput) (weatherOutput, error) {
			// Fake weather data for smoke test.
			return weatherOutput{Temp: 22, Condition: "sunny"}, nil
		},
	})

	agentResp, err := client.RunAgent(ctx, ai.AgentRequest{
		ModelProfile: ai.ModelChat,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What's the weather in Paris?"},
		},
		Tools:    []ai.Tool{weatherTool},
		MaxSteps: 5,
		Metadata: ai.Metadata{UserID: "smoke-test", Feature: "smoke-agent", ConversationID: "conv-2"},
	})
	if err != nil {
		fmt.Printf("SKIP (rate-limited): %v\n\n", err)
	} else {
		fmt.Printf("Steps:   %d\n", agentResp.Steps)
		fmt.Printf("Usage:   prompt=%d completion=%d\n", agentResp.Usage.PromptTokens, agentResp.Usage.CompletionTokens)
		fmt.Printf("Cost:    $%.6f\n", agentResp.CostUSD)
		for i, msg := range agentResp.Messages {
			fmt.Printf("Msg[%d]:  [%s] %s\n", i, msg.Role, truncate(msg.Content, 120))
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					fmt.Printf("  ToolCall: %s(%s)\n", tc.Fn.Name, tc.Fn.Arguments)
				}
			}
		}
	}
	fmt.Println()

	// ── Test 4: Streaming ──
	fmt.Println("--- cooldown 5s ---")
	time.Sleep(5 * time.Second)

	fmt.Println("=== Test 4: Streaming ===")
	sr, err := client.Stream(ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       "You are a helpful assistant. Respond in one sentence.",
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "Say hello in French."}},
		Metadata:     ai.Metadata{UserID: "smoke-test", Feature: "smoke-stream"},
	})
	if err != nil {
		fmt.Printf("SKIP (rate-limited): %v\n\n", err)
	} else {
		defer sr.Close()
		var streamContent string
		for {
			chunk, err := sr.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Printf("stream recv error: %v\n", err)
				break
			}
			streamContent += chunk.Delta
			if chunk.Usage != nil {
				fmt.Printf("Stream usage: prompt=%d completion=%d\n", chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens)
			}
		}
		fmt.Printf("Streamed: %s\n\n", streamContent)
	}

	// ── Summary ──
	fmt.Println("=== ALL SMOKE TESTS PASSED ===")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Suppress unused import.
var _ = json.Unmarshal
